// Package appinfo резолвит метаданные macOS-приложения по его bundle identifier
// (путь к .app, версия, имя) через Spotlight (mdfind) и Info.plist. Используется
// для наполнения словаря приложений и команды /info.
package appinfo

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Info — метаданные приложения, сохраняемые в словарь.
type Info struct {
	BundleID string `json:"bundle_id"`
	Path     string `json:"path"`
	Name     string `json:"name"`
	Version  string `json:"version"`
}

// Resolve находит метаданные по bundle id. Путь берётся из Spotlight, имя и версия
// из Info.plist приложения. Поля, которые не удалось получить, остаются пустыми —
// метод не считается «неуспешным», пока есть хотя бы bundle id.
func Resolve(bundleID string) Info {
	info := Info{BundleID: bundleID}
	path := spotlightPath(bundleID)
	if path == "" {
		return info
	}
	info.Path = path
	info.Name = plistValue(path, "CFBundleName")
	info.Version = plistValue(path, "CFBundleShortVersionString")
	return info
}

// spotlightPath возвращает путь к .app по bundle id через mdfind.
func spotlightPath(bundleID string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	query := "kMDItemCFBundleIdentifier == '" + bundleID + "'"
	out, err := exec.CommandContext(ctx, "/usr/bin/mdfind", query).Output()
	if err != nil {
		return ""
	}
	// mdfind может вернуть несколько путей — берём первый.
	for _, line := range strings.Split(string(out), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			return p
		}
	}
	return ""
}

// plistValue читает ключ из Info.plist приложения через defaults.
func plistValue(appPath, key string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	plist := appPath + "/Contents/Info"
	out, err := exec.CommandContext(ctx, "/usr/bin/defaults", "read", plist, key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
