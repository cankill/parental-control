package media

import (
	"slices"
	"testing"
)

func TestVideoCaptureArgsProduceFiveSecondTelegramCompatibleMP4(t *testing.T) {
	args := videoCaptureArgs("2", "/tmp/camera.mp4")
	for _, pair := range [][]string{
		{"-i", "2"}, {"-t", "5"}, {"-an", "-vf"},
		{"-c:v", "libx264"}, {"-pix_fmt", "yuv420p"},
		{"-movflags", "+faststart"},
	} {
		if !containsAdjacent(args, pair[0], pair[1]) {
			t.Fatalf("video args %q do not contain %q followed by %q", args, pair[0], pair[1])
		}
	}
	if args[len(args)-1] != "/tmp/camera.mp4" {
		t.Fatalf("destination = %q", args[len(args)-1])
	}
	if slices.Contains(args, "-frames:v") {
		t.Fatalf("video capture unexpectedly requests a single frame: %q", args)
	}
}

func containsAdjacent(values []string, first, second string) bool {
	for i := 0; i+1 < len(values); i++ {
		if values[i] == first && values[i+1] == second {
			return true
		}
	}
	return false
}
