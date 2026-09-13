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
	// Presence monitoring is disabled by default. When enabled, recent local
	// input is used first and camera frames are analyzed locally only after idle.
	PresenceEnabled          bool  `env:"PRESENCE_ENABLED" env-default:"false"`
	PresenceIntervalSeconds  int   `env:"PRESENCE_INTERVAL_SECONDS" env-default:"60"`
	PresenceIdleGraceSeconds int   `env:"PRESENCE_IDLE_GRACE_SECONDS" env-default:"120"`
	PresenceMissThreshold    int   `env:"PRESENCE_MISS_THRESHOLD" env-default:"3"`
	PresenceStartHour        int   `env:"PRESENCE_START_HOUR" env-default:"8"`
	PresenceEndHour          int   `env:"PRESENCE_END_HOUR" env-default:"18"`
	PresenceWorkDays         []int `env:"PRESENCE_WORK_DAYS" env-separator:"," env-default:"1,2,3,4,5"`
	// Face-touch detection runs automatically while presence is confirmed.
	// These deployment values tune sampling and notification deduplication;
	// there is intentionally no Telegram enable/disable command.
	FaceTouchIntervalSeconds  int `env:"FACE_TOUCH_INTERVAL_SECONDS" env-default:"10"`
	FaceTouchThresholdPercent int `env:"FACE_TOUCH_THRESHOLD_PERCENT" env-default:"80"`
	FaceTouchCooldownSeconds  int `env:"FACE_TOUCH_COOLDOWN_SECONDS" env-default:"30"`
}

// UrlPollInterval — интервал опроса URL браузера, минимум 1с, дефолт 3с.
func (e *Env) UrlPollInterval() time.Duration {
	if e.URLPollSeconds < 1 {
		return 3 * time.Second
	}
	return time.Duration(e.URLPollSeconds) * time.Second
}

func (e *Env) FaceTouchInterval() time.Duration {
	if e.FaceTouchIntervalSeconds < 5 {
		return 10 * time.Second
	}
	return time.Duration(e.FaceTouchIntervalSeconds) * time.Second
}

func (e *Env) FaceTouchThreshold() float64 {
	percent := e.FaceTouchThresholdPercent
	if percent < 50 || percent > 99 {
		percent = 80
	}
	return float64(percent) / 100
}

func (e *Env) FaceTouchCooldown() time.Duration {
	seconds := e.FaceTouchCooldownSeconds
	if seconds < 5 {
		seconds = 30
	}
	cooldown := time.Duration(seconds) * time.Second
	if cooldown < e.FaceTouchInterval() {
		return e.FaceTouchInterval()
	}
	return cooldown
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
