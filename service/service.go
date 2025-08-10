package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/Jubris-Knifes/wgj25-back/config"
	"github.com/Jubris-Knifes/wgj25-back/models"
	"github.com/Jubris-Knifes/wgj25-back/repository"
	"github.com/olahol/melody"
	"golang.org/x/sync/errgroup"
)

const (
	PlayerIDKey = "player_id"
)

var bidSelectedChan = make(chan models.BidSelected, 1)

type service struct {
	repo *repository.Repository
	log  *slog.Logger
	m    *melody.Melody
}

func New(logger *slog.Logger, repo *repository.Repository, m *melody.Melody) *service {
	return &service{
		repo: repo,
		log:  logger,
		m:    m,
	}

}

func (s *service) NewConnection(session *melody.Session) {
	s.log.InfoContext(session.Request.Context(),
		"New conncetion established",
		"remote_address",
		session.RemoteAddr().String(),
	)
}

func getAs[T any](log *slog.Logger, s *melody.Session, key string) (T, bool) {
	value, ok := s.Get(key)
	if !ok {
		var zero T
		return zero, false
	}

	valueT, ok := value.(T)
	if !ok {
		log.Error("type assertion failed", "key", key, "value", value, "type", fmt.Sprintf("%T", value))
	}

	return valueT, ok
}

func (s *service) ClosedConnection(session *melody.Session) {
	ctx := session.Request.Context()
	id, ok := getAs[int](s.log, session, PlayerIDKey)
	if !ok {
		s.log.ErrorContext(ctx, "failed to get player ID from session", "session_id", session.RemoteAddr().String())
		return
	}

	if err := s.repo.ClosePlayer(ctx, id); err != nil {
		s.log.ErrorContext(ctx, "failed to close player", "error", err, "player_id", id)
	}
}

func (s *service) HandleMessage(session *melody.Session, msg []byte) {
	var envelope models.EnvelopeIn
	if err := json.Unmarshal(msg, &envelope); err != nil {
		s.log.ErrorContext(session.Request.Context(), "failed to unmarshal message", "error", err)
		return
	}

	// Handle the message based on its type
	switch envelope.Type {
	case models.EventTypeSetName:
		s.handleSetNameEvent(session, envelope.EventData)
	case models.EventTypeBidSelected:
		s.handleBidSelectedEvent(session, envelope.EventData)
	default:
		s.log.WarnContext(session.Request.Context(), "unknown message type", "type", envelope.Type)
	}
}

func (s *service) handleBidSelectedEvent(session *melody.Session, eventData json.RawMessage) {
	var bidSelected models.BidSelected
	if err := json.Unmarshal(eventData, &bidSelected); err != nil {
		s.log.ErrorContext(session.Request.Context(), "failed to unmarshal bid_selected event", "error", err)
		return
	}

	bidSelectedChan <- bidSelected

}

func (s *service) handleSetNameEvent(session *melody.Session, eventData json.RawMessage) {
	ctx, cancel := context.WithTimeout(session.Request.Context(), 5*time.Second)
	defer cancel()

	s.log.DebugContext(ctx, "handling set_name event", "event_data", string(eventData))

	var setName models.SetName
	if err := json.Unmarshal(eventData, &setName); err != nil {
		s.log.ErrorContext(session.Request.Context(), "failed to unmarshal set_name event", "error", err)
		return
	}

	playerID, err := s.repo.NewPlayer(ctx, setName.Name)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to create new player", "error", err)
		return
	}

	session.Set(PlayerIDKey, playerID)

	s.log.DebugContext(ctx, "set_name event processed",
		"player_id", playerID,
		"name", setName.Name,
	)
	response := models.SetNameResponseEvent{
		Type: models.EventTypeSetNameResponse,
		EventData: models.SetNameResponse{
			AssignedPlayerID: playerID,
		},
	}

	errGroup := &errgroup.Group{}
	errGroup.Go(func() error {
		payload, err := json.Marshal(response)
		if err != nil {
			s.log.ErrorContext(ctx, "failed to marshal response", "error", err)
			return err
		}

		session.Write(payload)

		s.log.DebugContext(ctx, "set_name response sent",
			"player_id", playerID,
			"name", setName.Name,
		)

		return nil
	})

	errGroup.Go(func() error {
		response := models.PlayerJoinedEvent{
			Type: models.EventTypePlayerJoined,
			EventData: models.PlayerJoined{
				PlayerID: playerID,
				Name:     setName.Name,
			},
		}

		payload, err := json.Marshal(response)
		if err != nil {
			return err
		}

		if err := s.m.BroadcastOthers(payload, session); err != nil {
			s.log.ErrorContext(ctx, "failed to broadcast player joined", "error", err)
			return err
		}

		s.log.DebugContext(ctx, "player joined broadcast sent",
			"player_id", playerID, "name", setName.Name,
		)
		return nil
	})

	errGroup.Wait()

	if count, err := s.repo.GetActivePlayerCount(ctx); err != nil {
		s.log.ErrorContext(ctx, "failed to get active player count", "error", err)
	} else if count == 4 {
		go s.startRound()
	}
	// Process the set_name event
	s.log.InfoContext(session.Request.Context(), "set_name event received", "player_id", playerID, "name", setName.Name)
}

func (s *service) startRound() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.log.Info("Starting a new round")
	playerIDs, err := s.repo.GetActivePlayerIDs(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get active player IDs", "error", err)
		return
	}

	s.repo.DropPlayerHands(ctx)

	dealingCardsEvent := models.DealingCardsEvent{
		Type:      models.EventTypeDealingCards,
		EventData: models.DealingCards{},
	}

	payload, err := json.Marshal(dealingCardsEvent)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal dealing_cards event", "error", err)
		return
	}

	s.m.Broadcast(payload)

	playerCards := shuffleAndGiveCardsToPlayers(playerIDs)

	errGroup := &errgroup.Group{}
	for playerID, cards := range playerCards {
		errGroup.Go(func() error {
			s.log.InfoContext(context.Background(), "dealing cards", "player_id", playerID, "cards", cards)
			cardsDealtEvent := models.CardsDealtEvent{
				Type:      models.EventTypeCardsDealt,
				EventData: models.CardsDealt{Cards: cards},
			}

			if err := s.repo.SetPlayerHand(ctx, playerID, cards); err != nil {
				s.log.ErrorContext(ctx, "failed to set player hand", "error", err)
				return err
			}

			payload, err := json.Marshal(cardsDealtEvent)
			if err != nil {
				s.log.ErrorContext(context.Background(), "failed to marshal cards_dealt event", "error", err)
				return err
			}

			s.m.BroadcastFilter(payload, func(session *melody.Session) bool {
				pID, _ := getAs[int](s.log, session, PlayerIDKey)
				return pID == playerID
			})
			return nil
		})
	}
	errGroup.Wait()

	startingPlayer := rand.IntN(4)
	s.repo.SetCurrentPlayerID(ctx, playerIDs[startingPlayer])
	s.startTurn()
}

func (s *service) startTurn() {
	ctx := context.Background()

	currentPlayerID, err := s.repo.GetCurrentPlayerID(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get current player ID", "error", err)
		panic(err)
	}

	timeoutForChoice := time.Duration(config.Get().Timeouts.PlayerChooseBidMilliseconds) * time.Millisecond
	s.sendPlayerBidOfferEvent(ctx, currentPlayerID, timeoutForChoice)
	ctx, cancel := context.WithTimeout(ctx, timeoutForChoice)
	defer cancel()

	currentPlayerHand, err := s.repo.GetPlayerHand(ctx, currentPlayerID)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get current player hand", "error", err)
		panic(err)
	}

	choice := currentPlayerHand[rand.IntN(len(currentPlayerHand))]

	select {
	case playerChoice := <-bidSelectedChan:
		if playerChoice.IsRoundDone {
			// TODO: Handle end of round
		}
		choice = playerChoice.Card
	case <-ctx.Done():
	}

	s.sendhowChoiceEvent(choice, currentPlayerID)
}

func (s *service) sendhowChoiceEvent(choice models.Card, playerID int) {
	ctx := context.Background()

	s.log.DebugContext(ctx, "sending how choice event", "player_id", playerID, "card", choice)

	event := models.BidSelectedEvent{
		Type: models.EventTypeBidSelected,
		EventData: models.BidSelected{
			Card: choice,
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal bid_selected event", "error", err)
		return
	}

	s.broadcastToPlayerAndHub(payload, playerID)

	s.log.DebugContext(ctx, "bid_selected event sent", "player_id", playerID, "card", choice)

}

func (s *service) sendPlayerBidOfferEvent(ctx context.Context, playerID int, timeout time.Duration) {
	s.log.DebugContext(ctx, "sending player bid offer event", "player_id", playerID)

	event := models.ChooseBidEvent{
		Type: models.EventTypeChooseBid,
		EventData: models.ChooseBid{
			PlayerID: playerID,
			Timeout:  timeout.Milliseconds(),
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal player bid offer event", "error", err)
		return
	}

	s.broadcastToPlayerAndHub(payload, playerID)
}

func (s *service) broadcastToPlayerAndHub(payload []byte, playerID int) error {
	return s.m.BroadcastFilter(payload, func(session *melody.Session) bool {
		pID, ok := getAs[int](s.log, session, PlayerIDKey)
		return ok && pID == playerID
	})
}

func shuffleAndGiveCardsToPlayers(playerIDs []int) map[int][]models.Card {
	cardsForthisRound := slices.Clone(models.AvailableRealCards)

	fakeCards := slices.Clone(models.AvailableFakeCards)
	rand.Shuffle(len(fakeCards), func(i, j int) {
		fakeCards[i], fakeCards[j] = fakeCards[j], fakeCards[i]
	})

	fakeCount := 0
	selectedFakeCards := make([]models.Card, 0, 4)
	for fakeCount < 4 && len(fakeCards) > 0 {
		if !slices.Contains(selectedFakeCards, fakeCards[0]) {
			selectedFakeCards = append(selectedFakeCards, fakeCards[0])
			fakeCount++
		}
		fakeCards = fakeCards[1:]
	}

	cardsForthisRound = append(cardsForthisRound, selectedFakeCards...)

	rand.Shuffle(len(cardsForthisRound), func(i, j int) {
		cardsForthisRound[i], cardsForthisRound[j] = cardsForthisRound[j], cardsForthisRound[i]
	})

	playerHands := map[int][]models.Card{}
	for _, playerID := range playerIDs {
		for range 5 {
			playerHands[playerID] = append(playerHands[playerID], cardsForthisRound[0])
			cardsForthisRound = cardsForthisRound[1:]
		}
	}

	return playerHands
}
