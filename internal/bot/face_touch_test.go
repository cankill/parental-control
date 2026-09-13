package bot

import (
	"testing"
	"time"

	"parental-control/internal/facetouch"
)

func TestRenderFaceTouchCandidateIncludesPhotoAndLabels(t *testing.T) {
	candidate := facetouch.Candidate{
		Record: facetouch.Record{ID: "abc123", CapturedAt: time.Now(), Score: 0.84},
		Photo:  []byte("jpeg"),
	}
	message := renderFaceTouchCandidate(candidate)
	if len(message.Blocks) != 2 || message.Blocks[0].InputRichBlockPhoto == nil || message.Blocks[1].InputRichBlockButtons == nil {
		t.Fatalf("candidate message = %#v", message)
	}
	photo := message.Blocks[0].InputRichBlockPhoto
	if photo.Photo.Media != "attach://chin-abc123.jpg" || photo.Caption == nil || photo.Caption.Text.PlainText == "" {
		t.Fatalf("candidate photo = %#v", photo)
	}
	buttons := message.Blocks[1].InputRichBlockButtons.Buttons
	if len(buttons) != 2 || buttons[0].CallbackData != "\fface-touch-label|abc123:watch" || buttons[1].CallbackData != "\fface-touch-label|abc123:ignore" {
		t.Fatalf("candidate buttons = %#v", buttons)
	}
}

func TestParseFaceTouchLabel(t *testing.T) {
	id, label, ok := parseFaceTouchLabel("abc123:watch")
	if !ok || id != "abc123" || label != facetouch.LabelWatch {
		t.Fatalf("parsed label = %q %q %v", id, label, ok)
	}
	for _, invalid := range []string{"", "abc123", "abc123:pending", ":watch"} {
		if _, _, ok := parseFaceTouchLabel(invalid); ok {
			t.Fatalf("invalid label %q accepted", invalid)
		}
	}
}
