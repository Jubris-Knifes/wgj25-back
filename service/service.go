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

var (
	bidSelectedChan                = make(chan models.BidSelected, 1)
	offerSelectedChan              = make(chan models.PlayerOffer, 3)
	currentPlayerSelectedOfferChan = make(chan int, 1)
)

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

func (s *service) hasNextRound() bool {
	//TODO: when does game end?
	return true
}

func (s *service) endGame() {
	// TODO: define end game loop
	panic("unimplemented")
}

func (s *service) broadcastToHub(data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		s.log.ErrorContext(context.Background(), "failed to marshal broadcast payload", "error", err)
		return err
	}

	err = s.m.BroadcastFilter(payload, func(session *melody.Session) bool {
		_, ok := getAs[int](s.log, session, PlayerIDKey)
		return !ok
	})
	if err != nil {
		s.log.ErrorContext(context.Background(), "failed to broadcast to hub", "error", err)
		return err
	}

	return nil
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
	case models.EventTypeOfferSelected:
		s.handleOfferSelectedEvent(session, msg)
	case models.EventTypePlayerChooseOffer:
		s.handlePlayerChooseOfferEvent(session, envelope.EventData)
	default:
		s.log.WarnContext(session.Request.Context(), "unknown message type", "type", envelope.Type)
	}
}

func (s *service) handlePlayerChooseOfferEvent(session *melody.Session, eventData json.RawMessage) {
	var playerChooseOffer models.PlayerChooseOffer
	if err := json.Unmarshal(eventData, &playerChooseOffer); err != nil {
		s.log.ErrorContext(session.Request.Context(), "failed to unmarshal player_choose_offer event", "error", err)
		return
	}

	// Process the player choose offer event
	s.log.DebugContext(session.Request.Context(), "player_choose_offer event received", "offer", playerChooseOffer)

	currentPlayerSelectedOfferChan <- playerChooseOffer.PlayerID
}

func (s *service) handleOfferSelectedEvent(session *melody.Session, msg []byte) {
	playerID, ok := getAs[int](s.log, session, PlayerIDKey)

	if !ok {
		s.log.Error("Player Id not present on session for selecting offer")
		panic("session player id not found")
	}

	playerOffer := models.PlayerOffer{PlayerID: playerID}

	if err := json.Unmarshal(msg, &playerOffer.Card); err != nil {
		s.log.Error("failed to unmarshal player offer", "error", err)
		panic(err)
	}

	offerSelectedChan <- playerOffer
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
	s.startPlayerBid()
}

func (s *service) endOfRound() {
	ctx := context.Background()
	s.log.InfoContext(ctx, "Ending round")

	scores := s.getUpdatedScoreBoard()

	timeout := time.Duration(config.Get().Timeouts.EndOfRoundScreen) * time.Millisecond
	endOfRoundEvent := models.EndOfRoundEvent{
		Type: models.EventTypeEndOfRound,
		EventData: models.EndOfRound{
			Timeout: timeout.Milliseconds(),
		},
	}

	if err := s.broadcastToHub(endOfRoundEvent); err != nil {
		s.log.ErrorContext(ctx, "failed to broadcast end of round event", "error", err)
		panic(err)
	}
	time.Sleep(timeout)

	updateScoreTimeout := time.Duration(config.Get().Timeouts.UpdateScoreScreen) * time.Millisecond
	updateScoreEvent := models.UpdateScoreEvent{
		Type: models.EventTypeUpdateScore,
		EventData: models.UpdateScore{
			Timeout: updateScoreTimeout.Milliseconds(),
		},
	}

	for _, score := range scores {
		updateScoreEvent.EventData.Scores = append(updateScoreEvent.EventData.Scores, models.HandScore{
			PlayerID: score.PlayerID,
			Points:   score.RoundPoints,
			Cards:    score.Hand,
		})
	}

	if err := s.broadcastToHub(updateScoreEvent); err != nil {
		s.log.ErrorContext(ctx, "failed to broadcast update score event", "error", err)
		panic(err)
	}

	time.Sleep(updateScoreTimeout)

	sumScoreTimeout := time.Duration(config.Get().Timeouts.SumScore) * time.Millisecond

	sumScoreEvent := models.SumScoreEvent{
		Type: models.EventTypeSumScore,
		EventData: models.SumScore{
			Timeout: sumScoreTimeout.Milliseconds(),
		},
	}

	for _, score := range scores {
		sumScoreEvent.EventData.Scores = append(
			sumScoreEvent.EventData.Scores,
			models.ScoreChange{
				PlayerID: score.PlayerID,
				OldScore: score.OldPoints,
				NewScore: score.NewPoints,
			},
		)
	}

	if err := s.broadcastToHub(sumScoreEvent); err != nil {
		s.log.ErrorContext(ctx, "failed to broadcast sum score event", "error", err)
		panic(err)
	}

	time.Sleep(sumScoreTimeout)

	if !s.hasNextRound() {
		s.endGame()
	}

	prepareNextRoundTimeout := time.Duration(config.Get().Timeouts.PrepareForNextTurnMilliseconds) * time.Millisecond

	prepareNextRoundEvent := models.PrepareForNextTurnEvent{
		Type: models.EventTypePrepareForNextTurn,
		EventData: models.PrepareForNextTurn{
			Timeout: prepareNextRoundTimeout.Milliseconds(),
		},
	}

	payload, err := json.Marshal(prepareNextRoundEvent)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal prepare next round event", "error", err)
		panic(err)
	}

	if err := s.m.Broadcast(payload); err != nil {
		s.log.ErrorContext(ctx, "failed to broadcast prepare next round event", "error", err)
		panic(err)
	}

	time.Sleep(prepareNextRoundTimeout)

	s.startRound()
}

func (s *service) startPlayerBid() {
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
			s.endOfRound()
			return
		}
		choice = playerChoice.Card
	case <-ctx.Done():
	}

	s.sendPlayerBidWasSelectedEvent(choice, currentPlayerID)

	go s.startPlayersOffers(choice)
}

func calculateDiscountCausedByFakes(hand []models.Card) int {
	totalFakes := 0
	for _, card := range hand {
		if !card.IsReal {
			totalFakes++
		}
	}

	switch totalFakes {
	case 1:
		return config.Get().Points.FakeOne
	case 2:
		return config.Get().Points.FakeTwo
	case 3:
		return config.Get().Points.FakeThree
	case 4:
		return 0
	}

	panic("Unreachable code AHHHHHHHHH!!!!!!!!!!!!!!!!")
}

func isFakePoker(hand []models.Card) bool {
	totalFakes := 0
	for _, card := range hand {
		if !card.IsReal {
			totalFakes++
		}
	}

	return totalFakes == 4
}

func isPoker(hand []models.Card) bool {
	kinds := make([]int, 4)

	for _, card := range hand {
		kinds[card.Type]++
		if kinds[card.Type] == 4 {
			return true
		}
	}

	return false
}

func isOneOfEach(hand []models.Card) bool {
	kinds := make([]int, 4)
	usedKinds := 0

	for _, card := range hand {
		if kinds[card.Type] == 0 {
			usedKinds++
		}
		kinds[card.Type]++
	}

	return usedKinds == 4
}

func isFullHouse(hand []models.Card) bool {
	kinds := make([]int, 4)
	for _, card := range hand {
		kinds[card.Type]++
	}

	for _, count := range kinds {
		if count == 1 || count > 3 {
			return false
		}
	}

	return true
}

func isThreeOfAKind(hand []models.Card) bool {
	kinds := make([]int, 4)
	for _, card := range hand {
		kinds[card.Type]++
	}

	usedKinds := 0
	for _, count := range kinds {
		if count == 3 {
			usedKinds++
		}
	}

	return usedKinds == 1
}

func isTwoPair(hand []models.Card) bool {
	kinds := make([]int, 4)
	for _, card := range hand {
		kinds[card.Type]++
	}

	usedKinds := 0
	for _, count := range kinds {
		if count == 2 {
			usedKinds++
		}
	}

	return usedKinds == 2
}

func isPair(hand []models.Card) bool {
	kinds := make([]int, 4)
	for _, card := range hand {
		kinds[card.Type]++
	}

	usedKinds := 0
	for _, count := range kinds {
		if count == 2 {
			usedKinds++
		}
	}

	return usedKinds == 1
}

func calculateRoundPoints(hand []models.Card) int {
	points := calculateDiscountCausedByFakes(hand)

	switch {
	case isFakePoker(hand):
		points += config.Get().Points.FakePoker
	case isPoker(hand):
		points += config.Get().Points.Poker
	case isOneOfEach(hand):
		points += config.Get().Points.OneOfEach
	case isFullHouse(hand):
		points += config.Get().Points.FullHouse
	case isThreeOfAKind(hand):
		points += config.Get().Points.ThreeOfAKind
	case isTwoPair(hand):
		points += config.Get().Points.TwoPair
	case isPair(hand):
		points += config.Get().Points.Pair
	}

	return points
}

func (s *service) getUpdatedScoreBoard() []models.UpdatedScore {
	ctx := context.Background()
	scores, err := s.repo.GetPlayerScores(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get player score", "error", err)
		panic(err)
	}

	updatedScores := make([]models.UpdatedScore, 0, len(scores))
	for _, score := range scores {
		hand, err := s.repo.GetPlayerHand(ctx, score.PlayerID)
		if err != nil {
			s.log.ErrorContext(ctx, "failed to get player hand", "error", err, "player_id", score.PlayerID)
			panic(err)
		}

		roundPoints := calculateRoundPoints(hand)

		updatedScores = append(updatedScores, models.UpdatedScore{
			PlayerID:    score.PlayerID,
			RoundPoints: roundPoints,
			OldPoints:   score.Points,
			NewPoints:   score.Points + roundPoints,
			Hand:        hand,
		})
	}

	return updatedScores
}

func (s *service) sendOfferBackToPlayer(playerID int, card models.Card) {
	ctx := context.Background()

	event := models.OfferSelectedEvent{
		Type: models.EventTypeOfferSelected,
		EventData: models.OfferSelected{
			Card: card,
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal player offer", "error", err)
		panic(err)
	}

	err = s.m.BroadcastFilter(payload, func(session *melody.Session) bool {
		pID, ok := getAs[int](s.log, session, PlayerIDKey)
		return ok && pID == playerID
	})

	if err != nil {
		s.log.ErrorContext(ctx, "failed to broadcast player offer", "error", err)
		panic(err)
	}
}

func (s *service) startPlayersOffers(bid models.Card) {
	ctx := context.Background()
	currentPlayerID, err := s.repo.GetCurrentPlayerID(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get current player ID", "error", err)
		panic(err)
	}

	playerIDs, err := s.repo.GetActivePlayerIDs(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get active player IDs", "error", err)
		panic(err)
	}

	playerIDs = slices.DeleteFunc(playerIDs, func(id int) bool {
		return id == currentPlayerID
	})

	timeout := time.Duration(config.Get().Timeouts.PlayerChooseOfferMilliseconds) * time.Millisecond

	event := models.ChooseOfferEvent{
		Type: models.EventTypeChooseOffer,
		EventData: models.ChooseOffer{
			PlayerIDs: playerIDs,
			Timeout:   timeout.Milliseconds(),
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal choose_offer event", "error", err)
		panic(err)
	}

	err = s.m.BroadcastFilter(payload, func(session *melody.Session) bool {
		pID, ok := getAs[int](s.log, session, PlayerIDKey)
		return !ok || slices.Contains(playerIDs, pID)
	})

	if err != nil {
		s.log.ErrorContext(ctx, "failed to broadcast choose_offer event", "error", err)
		panic(err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	s.log.DebugContext(ctx, "choose_offer event broadcasted", "player_ids", playerIDs)

	playerDidOffer := make([]int, 0, len(playerIDs))
	playerOffersMap := make(map[int]models.Card, 3)
	for _, playerID := range playerIDs {
		playerHand, err := s.repo.GetPlayerHand(ctx, playerID)
		if err != nil {
			s.log.ErrorContext(ctx, "failed to get player hand", "error", err,
				"player_id", playerID,
			)
			panic(err)
		}

		playerOffersMap[playerID] = playerHand[rand.IntN(len(playerHand))]
		s.log.DebugContext(ctx, "selected player offer", "player_id", playerID, "offer", playerOffersMap[playerID])
	}

	for count := 0; count < len(playerIDs); count++ {
		select {
		case playerChoice := <-offerSelectedChan:
			playerOffersMap[playerChoice.PlayerID] = playerChoice.Card
			playerDidOffer = append(playerDidOffer, playerChoice.PlayerID)
			s.sendOfferBackToPlayer(playerChoice.PlayerID, playerChoice.Card)
			s.sendPlayerOfferEvent(playerDidOffer)
		case <-ctx.Done():
			s.log.DebugContext(ctx, "timeout reached for player offers")
			count = len(playerIDs) // Force exit the loop
			for _, playerID := range playerIDs {
				s.sendOfferBackToPlayer(playerID, playerOffersMap[playerID])
			}
		}
	}

	playerOffers := make([]models.PlayerOffer, 0, len(playerOffersMap))
	for playerID, card := range playerOffersMap {
		playerOffers = append(playerOffers, models.PlayerOffer{
			PlayerID: playerID,
			Card:     card,
		})
	}
	s.sendAllPlayerOffersEvent(playerOffers, currentPlayerID)

	s.startCurrentPlayerChoosesOffer(bid, playerOffers, currentPlayerID)
}

func (s *service) startCurrentPlayerChoosesOffer(bid models.Card, playerOffers []models.PlayerOffer, currentPlayerID int) {
	ctx := context.Background()

	s.log.DebugContext(ctx, "starting current player chooses offer", "player_id", currentPlayerID)

	timeout := time.Duration(config.Get().Timeouts.PlayerChooseOfferMilliseconds) * time.Millisecond

	playerChooseOfferEvent := models.SelectOfferChoicesEvent{
		Type: models.EventTypeSelectOfferChoices,
		EventData: models.SelectOfferChoices{
			Offers:  playerOffers,
			Timeout: timeout.Milliseconds(),
		},
	}

	payload, err := json.Marshal(playerChooseOfferEvent)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal select_offer_choices event", "error", err)
		panic(err)
	}

	if err := s.broadcastToPlayerAndHub(payload, currentPlayerID); err != nil {
		s.log.ErrorContext(ctx, "failed to broadcast select_offer_choices event", "error", err)
		panic(err)
	}

	s.log.DebugContext(ctx, "select_offer_choices event broadcasted", "offers", playerOffers, "current_player", currentPlayerID)

	selectedOfferIndex := rand.IntN(len(playerOffers))

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case playerID := <-currentPlayerSelectedOfferChan:
		selectedOfferIndex = slices.IndexFunc(playerOffers, func(offer models.PlayerOffer) bool {
			return offer.PlayerID == playerID
		})
	case <-ctx.Done():
	}

	offererID := playerOffers[selectedOfferIndex].PlayerID
	err = s.repo.SwapCardHolders(ctx, bid, playerOffers[selectedOfferIndex].Card, currentPlayerID, offererID)
	if err != nil {
		s.log.Error("Failed to swap card holders", "error", err)
		panic(err)
	}
	errGroup := &errgroup.Group{}
	//send update hand to current player
	errGroup.Go(func() error {
		cards, err := s.repo.GetPlayerHand(ctx, currentPlayerID)
		if err != nil {
			s.log.ErrorContext(ctx, "failed to get current player hand", "error", err)
			return err
		}

		updateCardsEvent := models.CardsUpdateEvent{
			Type: models.EventTypeCardsUpdate,
			EventData: models.CardsUpdate{
				Cards: cards,
			},
		}

		payload, err := json.Marshal(updateCardsEvent)
		if err != nil {
			s.log.ErrorContext(ctx, "failed to marshal update_cards event", "error", err)
			return err
		}

		err = s.m.BroadcastFilter(payload, func(session *melody.Session) bool {
			playerID, ok := getAs[int](s.log, session, PlayerIDKey)
			return ok && playerID == currentPlayerID
		})
		if err != nil {
			s.log.ErrorContext(ctx, "failed to broadcast update_cards event", "error", err)
			return err
		}

		return nil
	})

	//send update hand to offerer
	errGroup.Go(func() error {
		cards, err := s.repo.GetPlayerHand(ctx, offererID)
		if err != nil {
			s.log.ErrorContext(ctx, "failed to get current player hand", "error", err)
			return err
		}

		updateCardsEvent := models.CardsUpdateEvent{
			Type: models.EventTypeCardsUpdate,
			EventData: models.CardsUpdate{
				Cards: cards,
			},
		}

		payload, err := json.Marshal(updateCardsEvent)
		if err != nil {
			s.log.ErrorContext(ctx, "failed to marshal update_cards event", "error", err)
			return err
		}

		err = s.m.BroadcastFilter(payload, func(session *melody.Session) bool {
			playerID, ok := getAs[int](s.log, session, PlayerIDKey)
			return ok && playerID == offererID
		})
		if err != nil {
			s.log.ErrorContext(ctx, "failed to broadcast update_cards event", "error", err)
			return err
		}

		return nil
	})

	//notify hub
	errGroup.Go(func() error {
		timeout := time.Duration(config.Get().Timeouts.ShowSelectedOffer) * time.Millisecond

		event := models.SelectOfferChosenEvent{
			Type: models.EventTypeSelectOfferChosen,
			EventData: models.SelectOfferChosen{
				Timeout:  timeout.Milliseconds(),
				PlayerID: offererID,
			},
		}
		payload, err := json.Marshal(event)
		if err != nil {
			s.log.ErrorContext(ctx, "failed to marshal select_offer_chosen event", "error", err)
			return err
		}

		err = s.m.BroadcastFilter(payload, func(session *melody.Session) bool {
			_, ok := getAs[int](s.log, session, PlayerIDKey)
			return !ok
		})
		if err != nil {
			s.log.ErrorContext(ctx, "failed to broadcast select_offer_chosen event", "error", err)
			return err
		}

		time.Sleep(timeout)
		return nil
	})

	if err := errGroup.Wait(); err != nil {
		panic(err)
	}

	s.prepareForNextTurn()
}

func (s *service) prepareForNextTurn() {
	ctx := context.Background()

	currentPlayerID, err := s.repo.GetCurrentPlayerID(ctx)
	if err != nil {
		s.log.ErrorContext(context.Background(), "failed to get current player ID", "error", err)
		panic(err)
	}

	playerIDs, err := s.repo.GetActivePlayerIDs(ctx)
	if err != nil {
		s.log.ErrorContext(context.Background(), "failed to get active player IDs", "error", err)
		panic(err)
	}

	currentPlayerIndex := slices.Index(playerIDs, currentPlayerID)
	currentPlayerIndex = (currentPlayerIndex + 1) % len(playerIDs)

	currentPlayerID = playerIDs[currentPlayerIndex]
	if err := s.repo.SetCurrentPlayerID(ctx, currentPlayerID); err != nil {
		s.log.ErrorContext(ctx, "failed to set current player ID", "error", err)
		panic(err)
	}

	timeout := time.Duration(config.Get().Timeouts.PrepareForNextTurnMilliseconds) * time.Millisecond
	event := models.PrepareForNextTurnEvent{
		Type: models.EventTypePrepareForNextTurn,
		EventData: models.PrepareForNextTurn{
			Timeout:    timeout.Milliseconds(),
			NextBidder: currentPlayerID,
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal prepare_for_next_turn event", "error", err)
		panic(err)
	}

	if err := s.broadcastToPlayerAndHub(payload, currentPlayerID); err != nil {
		s.log.ErrorContext(ctx, "failed to broadcast prepare_for_next_turn event", "error", err)
		panic(err)
	}

	time.Sleep(timeout)

	s.startTurn()
}

func (s *service) sendAllPlayerOffersEvent(playerOffers []models.PlayerOffer, currentPlayerID int) {
	playerIDs := make([]int, 0, len(playerOffers))
	for _, offer := range playerOffers {
		playerIDs = append(playerIDs, offer.PlayerID)
	}
	s.sendPlayerOfferEvent(playerIDs)

	ctx := context.Background()
	s.log.DebugContext(ctx, "sending all player offers event", "player_offers", playerOffers)

	timeout := time.Duration(config.Get().Timeouts.OffersFinishedMilliseconds) * time.Millisecond

	event := models.OffersFinishedEvent{
		Type: models.EventTypeOfferSelected,
		EventData: models.OffersFinished{
			Timeout: timeout.Milliseconds(),
			Offers:  playerOffers,
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal offers_finished event", "error", err)
		panic(err)
	}

	s.broadcastToPlayerAndHub(payload, currentPlayerID)

	time.Sleep(time.Duration(config.Get().Timeouts.TimeBetweenActionsMilliseconds) * time.Millisecond)

	s.broadcastToPlayerAndHub(payload, currentPlayerID)
	s.log.DebugContext(ctx, "offers_finished event sent", "player_offers", playerOffers)
	time.Sleep(timeout)

}

func (s *service) sendPlayerOfferEvent(playerIDs []int) {
	ctx := context.Background()

	s.log.DebugContext(ctx, "sending player offer event", "player_ids", playerIDs)

	event := models.MadeOfferEvent{
		Type: models.EventTypeMadeOffer,
		EventData: models.MadeOffer{
			PlayerIDs: playerIDs,
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal player_offer event", "error", err)
		panic(err)
	}

	err = s.m.BroadcastFilter(payload, func(session *melody.Session) bool {
		_, ok := getAs[int](s.log, session, PlayerIDKey)
		return !ok
	})
	if err != nil {
		s.log.ErrorContext(ctx, "failed to broadcast player_offer event", "error", err)
		panic(err)
	}

	s.log.DebugContext(ctx, "player_offer event sent", "player_ids", playerIDs)
}

func (s *service) sendPlayerBidWasSelectedEvent(choice models.Card, playerID int) {
	ctx := context.Background()

	timeout := time.Duration(config.Get().Timeouts.ShowBidMilliseconds) * time.Millisecond
	showBackCardEvent := models.ShowBackOfCardBidEvent{
		Type: models.EventTypeShowBackOfCardBid,
		EventData: models.ShowBackOfCardBid{
			Timetout: timeout.Milliseconds(),
		},
	}

	payload, err := json.Marshal(showBackCardEvent)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to marshal show back of card event", "error", err)
		panic(err)
	}

	err = s.m.BroadcastFilter(payload, func(session *melody.Session) bool {
		_, ok := getAs[int](s.log, session, PlayerIDKey)
		return !ok
	})

	time.Sleep(timeout)
	s.log.DebugContext(ctx, "sending how choice event", "player_id", playerID, "card", choice)

	timeout = time.Duration(config.Get().Timeouts.ShowBidMilliseconds) * time.Millisecond

	event := models.ShowBidSelectedEvent{
		Type: models.EventTypeBidSelected,
		EventData: models.ShowBidSelected{
			Card:    choice,
			Timeout: timeout.Milliseconds(),
		},
	}

	payload, err = json.Marshal(event)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to marshal bid_selected event", "error", err)
		return
	}

	s.broadcastToPlayerAndHub(payload, playerID)

	s.log.DebugContext(ctx, "bid_selected event sent", "player_id", playerID, "card", choice)

	time.Sleep(timeout)
}

func canFinishRound(hand []models.Card) bool {
	switch {
	case isFakePoker(hand), isOneOfEach(hand), isPoker(hand):
		return true
	}

	return false

}

func (s *service) sendPlayerBidOfferEvent(ctx context.Context, playerID int, timeout time.Duration) {

	hand, err := s.repo.GetPlayerHand(ctx, playerID)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get player hand",
			"player_id", playerID,
			"error", err,
		)
		panic(err)
	}

	s.log.DebugContext(ctx, "sending player bid offer event", "player_id", playerID)
	event := models.ChooseBidEvent{
		Type: models.EventTypeChooseBid,
		EventData: models.ChooseBid{
			PlayerID:       playerID,
			Timeout:        timeout.Milliseconds(),
			CanFinishRound: canFinishRound(hand),
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
