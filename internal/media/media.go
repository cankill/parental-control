// Package media снимает фото с камеры и записывает звук с микрофона через ffmpeg
// (avfoundation). Используется командами /photo и /record. Требует TCC-разрешений
// Camera/Microphone (выдаются диалогом один раз) — см. entitlements в деплое.
package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// outputDir — каталог для временных медиа-файлов (тот же, что и для скриншотов).
const outputDir = "/tmp/pc"

// ffmpegPath ищет ffmpeg в PATH и типовых местах установки на macOS.
func ffmpegPath() (string, error) {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}
	for _, p := range []string{"/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("ffmpeg not found")
}

// CapturePhoto снимает один кадр с камеры по умолчанию (avfoundation device "0")
// и возвращает путь к JPEG. Вызывающий обязан удалить файл после отправки.
func CapturePhoto() (string, error) {
	ff, err := ffmpegPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", err
	}
	fname := filepath.Join(outputDir, fmt.Sprintf("photo-%d.jpg", time.Now().UnixNano()))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// -f avfoundation -i "0" — видеоустройство 0 (камера); один кадр.
	cmd := exec.CommandContext(ctx, ff,
		"-y", "-f", "avfoundation", "-video_size", "1280x720", "-i", "0",
		"-frames:v", "1", "-q:v", "5", fname)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg photo failed: %v: %s", err, tail(out))
	}
	return fname, nil
}

// RecordAudio записывает seconds секунд звука с микрофона по умолчанию
// (avfoundation ":0") в AAC/m4a и возвращает путь. seconds ограничивается [1,60].
func RecordAudio(seconds int) (string, error) {
	if seconds < 1 {
		seconds = 1
	}
	if seconds > 60 {
		seconds = 60
	}
	ff, err := ffmpegPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", err
	}
	fname := filepath.Join(outputDir, fmt.Sprintf("audio-%d.m4a", time.Now().UnixNano()))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds+10)*time.Second)
	defer cancel()
	// -f avfoundation -i ":0" — аудиоустройство 0 (микрофон); -t длительность.
	cmd := exec.CommandContext(ctx, ff,
		"-y", "-f", "avfoundation", "-i", ":0",
		"-t", fmt.Sprintf("%d", seconds), fname)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg audio failed: %v: %s", err, tail(out))
	}
	return fname, nil
}

// tail возвращает хвост вывода ffmpeg (для сообщения об ошибке, без спама).
func tail(b []byte) string {
	const max = 300
	if len(b) > max {
		return string(b[len(b)-max:])
	}
	return string(b)
}
