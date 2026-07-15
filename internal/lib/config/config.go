package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Env struct {
	BotToken string `env:"TG_BOT_TOKEN" env-requires:"true"`
	// AdminIDs — Telegram user id, которым разрешён доступ к боту (whitelist).
	// Задаётся как TG_ADMIN_IDS="123,456"; при отсутствии используется дефолт в bot.
	AdminIDs []int64 `env:"TG_ADMIN_IDS" env-separator:","`
	// URLPollSeconds — период опроса URL активного браузера для трекинга доменов.
	// 0 (по умолчанию) означает 3с — задаётся в UrlPollInterval().
	URLPollSeconds int `env:"URL_POLL_SECONDS"`
}

// UrlPollInterval — интервал опроса URL браузера, минимум 1с, дефолт 3с.
func (e *Env) UrlPollInterval() time.Duration {
	if e.URLPollSeconds < 1 {
		return 3 * time.Second
	}
	return time.Duration(e.URLPollSeconds) * time.Second
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
