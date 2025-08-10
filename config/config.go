package config

import (
	"sync"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type (
	zrok struct {
		UseReserved  bool   `env:"ZROK_USE_RESERVED" envDefault:"false"`
		ReservedName string `env:"ZROK_RESERVED_NAME"`
	}

	timeouts struct {
		PlayerChooseBidMilliseconds    int `env:"TIMEOUT_PLAYER_CHOOSE_BID_MILLISECONDS" envDefault:"5000"`
		ShowBidMilliseconds            int `env:"TIMEOUT_SHOW_BID_MILLISECONDS" envDefault:"1500"`
		PlayerChooseOfferMilliseconds  int `env:"TIMEOUT_PLAYER_CHOOSE_OFFER_MILLISECONDS" envDefault:"5000"`
		ShowOfferMilliseconds          int `env:"TIMEOUT_SHOW_OFFER_MILLISECONDS" envDefault:"2500"`
		TimeBetweenActionsMilliseconds int `env:"TIMEOUT_BETWEEN_ACTIONS_MILLISECONDS" envDefault:"1000"`
		OffersFinishedMilliseconds     int `env:"TIMEOUT_OFFERS_FINISHED_MILLISECONDS" envDefault:"3000"`
		ShowSelectedOffer              int `env:"TIMEOUT_SHOW_SELECTED_OFFER_MILLISECONDS" envDefault:"2000"`
		PrepareForNextTurnMilliseconds int `env:"TIMEOUT_PREPARE_FOR_NEXT_TURN_MILLISECONDS" envDefault:"2000"`

		EndOfRoundScreen  int `env:"TIMEOUT_END_OF_ROUND_SCREEN" envDefault:"3000"`
		UpdateScoreScreen int `env:"TIMEOUT_UPDATE_SCORE_SCREEN" envDefault:"6500"`
		SumScore          int `env:"TIMEOUT_SUMSCORE" envDefault:"4000"`
	}

	points struct {
		FakePoker    int `env:"POINTS_FAKE_POKER" envDefault:"8000"`
		Poker        int `env:"POINTS_POKER" envDefault:"5000"`
		OneOfEach    int `env:"POINTS_ONE_OF_EACH" envDefault:"4000"`
		FullHouse    int `env:"POINTS_FULL_HOUSE" envDefault:"3500"`
		ThreeOfAKind int `env:"POINTS_THREE_OF_A_KIND" envDefault:"3000"`
		TwoPair      int `env:"POINTS_TWO_PAIR" envDefault:"2500"`
		Pair         int `env:"POINTS_PAIR" envDefault:"2000"`

		FakeOne   int `env:"POINTS_FAKE_ONE" envDefault:"-250"`
		FakeTwo   int `env:"POINTS_FAKE_TWO" envDefault:"-1000"`
		FakeThree int `env:"POINTS_FAKE_THREE" envDefault:"-2500"`
	}

	config struct {
		MaxPlayers int `env:"MAX_PLAYERS" envDefault:"100"`
		Zrok       zrok
		Port       int `env:"PORT" envDefault:"8080"`
		Timeouts   timeouts
		Points     points
	}
)

var (
	conf config
	once = &sync.Once{}
)

func Get() config {
	once.Do(func() {
		godotenv.Load()
		if err := env.Parse(&conf); err != nil {
			panic(err)
		}
	})

	return conf
}
