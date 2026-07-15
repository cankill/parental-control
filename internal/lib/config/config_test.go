package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUrlPollInterval(t *testing.T) {
	cases := []struct {
		seconds int
		want    time.Duration
	}{
		{0, 3 * time.Second},   // дефолт
		{-5, 3 * time.Second},  // невалидное → дефолт
		{1, 1 * time.Second},   // минимум
		{10, 10 * time.Second}, // явное значение
	}
	for _, c := range cases {
		e := &Env{URLPollSeconds: c.seconds}
		if got := e.UrlPollInterval(); got != c.want {
			t.Errorf("UrlPollInterval(%d) = %v, want %v", c.seconds, got, c.want)
		}
	}
}

// AdminIDs должны парситься из TG_ADMIN_IDS через запятую (несколько пользователей).
func TestMustLoadParsesMultipleAdmins(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := "TG_BOT_TOKEN=dummy-token\nTG_ADMIN_IDS=183358896,1624096159\nURL_POLL_SECONDS=5\n"
	if err := os.WriteFile(envFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PARENTAL_CONTROL_ENV", envFile)

	env := MustLoad()
	if env.BotToken != "dummy-token" {
		t.Errorf("BotToken = %q", env.BotToken)
	}
	if len(env.AdminIDs) != 2 || env.AdminIDs[0] != 183358896 || env.AdminIDs[1] != 1624096159 {
		t.Errorf("AdminIDs = %v, want [183358896 1624096159]", env.AdminIDs)
	}
	if env.UrlPollInterval() != 5*time.Second {
		t.Errorf("interval = %v, want 5s", env.UrlPollInterval())
	}
}
