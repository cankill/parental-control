package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Env struct {
	BotToken string `env:"TG_BOT_TOKEN" env-requires:"true"`
	// AdminIDs — Telegram user id, которым разрешён доступ к боту (whitelist).
	// Задаётся как TG_ADMIN_IDS="123,456"; при отсутствии используется дефолт в bot.
	AdminIDs []int64 `env:"TG_ADMIN_IDS" env-separator:","`
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
