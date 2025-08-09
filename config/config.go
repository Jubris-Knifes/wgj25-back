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
	config struct {
		MaxPlayers int `env:"MAX_PLAYERS" envDefault:"100"`
		Zrok       zrok
		Port       int `env:"PORT" envDefault:"8080"`
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
