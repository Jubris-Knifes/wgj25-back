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
	EventTypeChooseOffer   EventType = "choose_Offer"
	EventTypeOfferSelected EventType = "Offer_selected"
)

type (
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
	EventTypeChooseBid   EventType = "choose_bid"
	EventTypeBidSelected EventType = "bid_selected"
)

type (
	ChooseBidEvent = Envelope[ChooseBid]

	ChooseBid struct {
		PlayerID int   `json:"player_id"`
		Timeout  int64 `json:"timeout"`
	}

	BidSelectedEvent = Envelope[BidSelected]

	ShowBidSelectedEvent = Envelope[ShowBidSelected]
	ShowBidSelected      struct {
		Card    Card  `json:"card"`
		Timeout int64 `json:"is_round_over"`
	}

	BidSelected struct {
		Card        Card `json:"card"`
		IsRoundDone bool `json:"is_round_over"`
	}
)

const (
	EventTypeDealingCards EventType = "dealing_cards"
	EventTypeCardsDealt   EventType = "cards_dealt"
)

type (
	DealingCardsEvent = Envelope[DealingCards]

	DealingCards struct{}

	CardsDealtEvent = Envelope[CardsDealt]

	CardsDealt struct {
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
