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
	"regexp"
	"strings"
	"time"
)

// outputDir — каталог для временных медиа-файлов (тот же, что и для скриншотов).
const outputDir = "/tmp/pc"

// signedFFmpeg — копия ffmpeg, переподписанная нашей identity (ParentControlSigning)
// с entitlements camera/mic. Нужна, потому что camera-грант TCC привязан к процессу,
// открывающему камеру; brew-ffmpeg подписан ad-hoc и НЕ наследует грант агента.
// Разворачивается и подписывается при деплое (init.sls / codesign.sh).
const signedFFmpeg = "/opt/parentcontrol/ffmpeg"

// ffmpegPath возвращает путь к ffmpeg: сначала нашу подписанную копию (для доступа
// к камере/микрофону через TCC-грант нашей identity), затем системный ffmpeg.
func ffmpegPath() (string, error) {
	candidates := []string{signedFFmpeg, "/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg"}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("ffmpeg not found")
}

// cameraIndex определяет индекс avfoundation-камеры (номер устройства меняется от
// Mac к Maк, и это НЕ всегда 0 — под 0 может быть "Capture screen"). Парсит
// -list_devices и возвращает первый видеодевайс, не являющийся screen-capture.
// При неудаче возвращает "0" как безопасный дефолт.
func cameraIndex(ff string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, ff, "-f", "avfoundation", "-list_devices", "true", "-i", "").CombinedOutput()

	inVideo := false
	// Строки вида: [AVFoundation ...] [0] FaceTime HD Camera (Built-in)
	re := regexp.MustCompile(`\[(\d+)\]\s+(.*)`)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "video devices") {
			inVideo = true
			continue
		}
		if strings.Contains(line, "audio devices") {
			inVideo = false
			continue
		}
		if !inVideo {
			continue
		}
		if m := re.FindStringSubmatch(line); m != nil {
			name := strings.ToLower(m[2])
			if strings.Contains(name, "capture screen") {
				continue // это захват экрана, не камера
			}
			return m[1] // первый настоящий видеодевайс (камера)
		}
	}
	return "0"
}

// CapturePhoto снимает один кадр с камеры и возвращает путь к JPEG. Вызывающий
// обязан удалить файл после отправки.
func CapturePhoto() (string, error) {
	ff, err := ffmpegPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", err
	}
	fname := filepath.Join(outputDir, fmt.Sprintf("photo-%d.jpg", time.Now().UnixNano()))
	dev := cameraIndex(ff)

	shoot := func() ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		// -i <dev> — индекс камеры (определён cameraIndex). -update 1 обязателен для
		// одиночного JPEG через image2-муксер, иначе ffmpeg ругается/ждёт паттерн
		// имени. НЕ навязываем -video_size/-pixel_format: камера отдаёт нативный
		// режим (uyvy422), жёсткий формат ломает захват при Continuity Camera.
		cmd := exec.CommandContext(ctx, ff,
			"-y", "-f", "avfoundation", "-framerate", "30", "-i", dev,
			"-frames:v", "1", "-update", "1", "-q:v", "5", fname)
		return cmd.CombinedOutput()
	}

	// Камера на macOS эксклюзивна: если её держит Zoom/Teams/видеозвонок, ffmpeg
	// возвращает "Input/output error". Пробуем несколько раз — камера могла
	// освободиться. Микрофон, в отличие от камеры, шарится, поэтому /record такого
	// не требует.
	var out []byte
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}
		if out, err = shoot(); err == nil {
			return fname, nil
		}
	}

	if strings.Contains(string(out), "Input/output error") {
		return "", fmt.Errorf("camera busy — close apps using it (Zoom/Teams/browser call) and retry")
	}
	return "", fmt.Errorf("ffmpeg photo failed: %v: %s", err, tail(out))
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
