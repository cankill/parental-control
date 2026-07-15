// Package browser достаёт URL активной вкладки поддерживаемых браузеров через
// AppleScript (osascript). Используется для команды /url и трекинга доменов.
package browser

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// appName — имя приложения для AppleScript "tell application". tabPhrase — как
// адресуется активная вкладка (Chrome-семейство: "active tab", Safari: "current tab").
type spec struct {
	appName   string
	tabPhrase string
}

// supported сопоставляет macOS bundle identifier с AppleScript-спецификой браузера.
var supported = map[string]spec{
	"com.google.Chrome":                {"Google Chrome", "active tab"},
	"com.apple.Safari":                 {"Safari", "current tab"},
	"ru.yandex.desktop.yandex-browser": {"Yandex", "active tab"},
	"com.microsoft.edgemac":            {"Microsoft Edge", "active tab"},
	"com.brave.Browser":                {"Brave Browser", "active tab"},
}

// IsBrowser сообщает, поддерживается ли получение URL для данного bundle id.
func IsBrowser(bundleID string) bool {
	_, ok := supported[bundleID]
	return ok
}

// ActiveTabURL возвращает URL активной вкладки браузера с данным bundle id.
// Пустая строка без ошибки — браузер не поддержан или вкладки нет.
func ActiveTabURL(bundleID string) (string, error) {
	sp, ok := supported[bundleID]
	if !ok {
		return "", nil
	}
	script := fmt.Sprintf("tell application %q to get URL of %s of front window", sp.appName, sp.tabPhrase)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script).Output()
	if err != nil {
		// Нет разрешения Automation, окно не открыто, таймаут — не считаем фатальным.
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// FrontmostBundleID возвращает bundle id активного (frontmost) приложения.
func FrontmostBundleID() (string, error) {
	const script = `tell application "System Events" to get bundle identifier of first application process whose frontmost is true`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// FrontmostBrowserURL возвращает URL активной вкладки, если frontmost-приложение —
// поддерживаемый браузер; иначе пустую строку (без ошибки).
func FrontmostBrowserURL() (string, error) {
	bundleID, err := FrontmostBundleID()
	if err != nil {
		return "", err
	}
	if !IsBrowser(bundleID) {
		return "", nil
	}
	return ActiveTabURL(bundleID)
}

// Domain извлекает хост (домен) из URL. Пустая строка, если распарсить не удалось.
func Domain(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.TrimPrefix(u.Host, "www.")
}
