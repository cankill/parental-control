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
	summary := facetouch.Summary{Dataset: 120, DatasetLabeled: 93}
	message := renderFaceTouchCandidate(candidate, summary)
	if len(message.Blocks) != 2 || message.Blocks[0].InputRichBlockPhoto == nil || message.Blocks[1].InputRichBlockButtons == nil {
		t.Fatalf("candidate message = %#v", message)
	}
	photo := message.Blocks[0].InputRichBlockPhoto
	if photo.Photo.Media != "attach://chin-abc123.jpg" || photo.Caption == nil || photo.Caption.Text.PlainText == "" {
		t.Fatalf("candidate photo = %#v", photo)
	}
	if photo.Caption.Text.PlainText != "Pinch near chin\nScore: 84%\nLabeled: 93/120\nRemaining: 27\nIs this a hair-plucking pose?" {
		t.Fatalf("candidate caption = %q", photo.Caption.Text.PlainText)
	}
	buttons := message.Blocks[1].InputRichBlockButtons.Buttons
	if len(buttons) != 2 || buttons[0].CallbackData != "\fface-touch-label|abc123:watch" || buttons[1].CallbackData != "\fface-touch-label|abc123:ignore" {
		t.Fatalf("candidate buttons = %#v", buttons)
	}
}

func TestRenderLabeledFaceTouchRemovesChoiceAndShowsStatusAtBottomRight(t *testing.T) {
	tests := []struct {
		label facetouch.Label
		icon  string
	}{
		{label: facetouch.LabelWatch, icon: "✅"},
		{label: facetouch.LabelIgnore, icon: "❌"},
	}
	for _, test := range tests {
		record := facetouch.Record{ID: "abc123", Score: 0.84, Label: test.label}
		summary := facetouch.Summary{Dataset: 120, DatasetLabeled: 94}
		message := renderLabeledFaceTouch(record, []byte("jpeg"), summary)
		if len(message.Blocks) != 2 || message.Blocks[0].InputRichBlockPhoto == nil {
			t.Fatalf("labeled message = %#v", message)
		}
		photo := message.Blocks[0].InputRichBlockPhoto
		if photo.Caption.Text.PlainText != "Pinch near chin\nScore: 84%\nLabeled: 94/120\nRemaining: 26" {
			t.Fatalf("labeled caption = %q", photo.Caption.Text.PlainText)
		}
		status := message.Blocks[1].InputRichBlockButtons
		if status == nil || status.Align != "right" || len(status.Buttons) != 1 {
			t.Fatalf("status block = %#v", status)
		}
		button := status.Buttons[0]
		if button.Text.PlainText != test.icon || button.Disabled == nil || button.CallbackData != "" {
			t.Fatalf("status button = %#v", button)
		}
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
