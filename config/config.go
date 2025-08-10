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
		PlayerChooseBidMilliseconds   int `env:"TIMEOUT_PLAYER_CHOOSE_BID_MILLISECONDS" envDefault:"5000"`
		ShowBidMilliseconds           int `env:"TIMEOUT_SHOW_BID_MILLISECONDS" envDefault:"1500"`
		PlayerChooseOfferMilliseconds int `env:"TIMEOUT_PLAYER_CHOOSE_OFFER_MILLISECONDS" envDefault:"5000"`
		ShowOfferMilliseconds         int `env:"TIMEOUT_SHOW_OFFER_MILLISECONDS" envDefault:"2500"`
	}

	config struct {
		MaxPlayers int `env:"MAX_PLAYERS" envDefault:"100"`
		Zrok       zrok
		Port       int `env:"PORT" envDefault:"8080"`
		Timeouts   timeouts
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
