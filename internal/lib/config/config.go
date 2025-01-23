package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Env struct {
	BotToken string `env:"BOT_TOKEN" env-requires:"true"`
}

func MustLoad() *Env {
	env := Env{}
	err := cleanenv.ReadConfig("./env", &env)
	if err != nil {
		fmt.Printf("Can't parse .env file: %s", err.Error())
		os.Exit(1)
	}

	return &env
}
