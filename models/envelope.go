package models

import "encoding/json"

type (
	EventType string

	Envelope[EventData any] struct {
		Type      EventType `json:"type"`
		EventData EventData `json:"event_data"`
	}
	EnvelopeIn struct {
		Type      EventType       `json:"type"`
		EventData json.RawMessage `json:"event_data"`
	}
)

const (
	EventTypeEndOfRound  EventType = "end_of_round"
	EventTypeUpdateScore EventType = "update_score"
	EventTypeSumScore    EventType = "sum_score"
)

type (
	EndOfRoundEvent = Envelope[EndOfRound]
	EndOfRound      struct {
		Timeout int64 `json:"timeout"`
	}

	UpdateScoreEvent = Envelope[UpdateScore]
	UpdateScore      struct {
		Timeout int64       `json:"timeout"`
		Scores  []HandScore `json:"scores"`
	}
	HandScore struct {
		PlayerID int    `json:"player_id"`
		Points   int    `json:"points"`
		Cards    []Card `json:"cards"`
	}

	SumScoreEvent = Envelope[SumScore]
	SumScore      struct {
		Timeout int64         `json:"timeout"`
		Scores  []ScoreChange `json:"scores"`
	}
	ScoreChange struct {
		PlayerID int `json:"player_id"`
		OldScore int `json:"old_score"`
		NewScore int `json:"new_score"`
	}
)

const (
	EventTypePrepareForNextTurn EventType = "prepare_for_next_turn"
)

type (
	PrepareForNextTurnEvent = Envelope[PrepareForNextTurn]
	PrepareForNextTurn      struct {
		Timeout    int64 `json:"timeout"`
		NextBidder int   `json:"next_bidder"`
	}
)

const (
	EventTypeChooseOffer        EventType = "choose_Offer"
	EventTypeOfferSelected      EventType = "Offer_selected"
	EventTypeMadeOffer          EventType = "made_offer"
	EventTypeOffersFinished     EventType = "offers_finished"
	EventTypeSelectOfferChoices EventType = "select_offer_choices"
	EventTypeSelectOfferChosen  EventType = "select_offer_chosen"
	EventTypePlayerChooseOffer  EventType = "player_choose_offer"
)

type PlayerOffer struct {
	PlayerID int  `json:"player_id"`
	Card     Card `json:"card"`
}

type (
	PlayerChooseOfferEvent = Envelope[PlayerChooseOffer]
	PlayerChooseOffer      struct {
		PlayerID int `json:"player_id"`
	}

	SelectOfferChosenEvent = Envelope[SelectOfferChosen]
	SelectOfferChosen      struct {
		Timeout  int64 `json:"timeout"`
		PlayerID int   `json:"player_id"`
	}

	SelectOfferChoicesEvent = Envelope[SelectOfferChoices]
	SelectOfferChoices      struct {
		Offers  []PlayerOffer `json:"offers"`
		Timeout int64         `json:"timeout"`
	}
	OffersFinishedEvent = Envelope[OffersFinished]
	OffersFinished      struct {
		Offers  []PlayerOffer `json:"offers"`
		Timeout int64         `json:"timeout"`
	}

	MadeOfferEvent = Envelope[MadeOffer]
	MadeOffer      struct {
		PlayerIDs []int `json:"player_ids"`
	}

	ChooseOfferEvent = Envelope[ChooseOffer]

	ChooseOffer struct {
		PlayerIDs []int `json:"player_ids"`
		Timeout   int64 `json:"timeout"`
	}

	OfferSelectedEvent = Envelope[OfferSelected]

	OfferSelected struct {
		Card Card `json:"card"`
	}
)

const (
	EventTypeChooseBid         EventType = "choose_bid"
	EventTypeShowBackOfCardBid EventType = "show_back_of_card_bid"
	EventTypeBidSelected       EventType = "bid_selected"
)

type (
	ShowBackOfCardBidEvent = Envelope[ShowBackOfCardBid]

	ShowBackOfCardBid struct {
		Timetout int64 `json:"timeout"`
	}

	ChooseBidEvent = Envelope[ChooseBid]

	ChooseBid struct {
		PlayerID       int   `json:"player_id"`
		Timeout        int64 `json:"timeout"`
		CanFinishRound bool  `json:"can_finish_round"`
	}

	BidSelectedEvent = Envelope[BidSelected]

	ShowBidSelectedEvent = Envelope[ShowBidSelected]
	ShowBidSelected      struct {
		Card    Card  `json:"card"`
		Timeout int64 `json:"timeout"`
	}

	BidSelected struct {
		Card        Card `json:"card"`
		IsRoundDone bool `json:"is_round_over"`
	}
)

const (
	EventTypeCardsUpdate  EventType = "cards_update"
	EventTypeDealingCards EventType = "dealing_cards"
	EventTypeCardsDealt   EventType = "cards_dealt"
)

type (
	CardsUpdateEvent = Envelope[CardsUpdate]
	CardsUpdate      struct {
		Cards []Card `json:"cards"`
	}

	DealingCardsEvent = Envelope[DealingCards]
	DealingCards      struct{}

	CardsDealtEvent = Envelope[CardsDealt]
	CardsDealt      struct {
		Cards []Card `json:"cards"`
	}
)

const (
	EventTypeSetName         EventType = "set_name_request"
	EventTypeSetNameResponse EventType = "set_name_response"
)

type (
	SetNameEvent = Envelope[SetName]

	SetName struct {
		Name string `json:"name"`
	}

	SetNameResponseEvent = Envelope[SetNameResponse]

	SetNameResponse struct {
		AssignedPlayerID int `json:"assigned_player_id"`
	}
)

const EventTypePlayerJoined EventType = "player_joined"

type (
	PlayerJoinedEvent = Envelope[PlayerJoined]

	PlayerJoined struct {
		PlayerID int    `json:"player_id"`
		Name     string `json:"name"`
	}
)
