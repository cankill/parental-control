package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Env struct {
	BotToken string `env:"TG_BOT_TOKEN" env-requires:"true"`
}

func MustLoad() *Env {
	envFile := os.Getenv("PARENTAL_CONTROL_ENV")
	env := Env{}
	err := cleanenv.ReadConfig(envFile, &env)
	if err != nil {
		fmt.Printf("Can't parse .env file: %s", err.Error())
		os.Exit(1)
	}

	return &env
}
