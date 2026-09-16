package bot

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"parental-control/internal/facetouch"

	"github.com/go-telegram/bot/models"
)

const faceTouchLabelAction = "face-touch-label"

func (h *handlerRegistry) registerFaceTouchHandlers() {
	h.command("chin", h.sendFaceTouchStats)
	h.callback(faceTouchLabelAction, h.labelFaceTouch)
}

func (h *handlerRegistry) forwardFaceTouchEvents(ctx context.Context, admins []int64, events <-chan facetouch.Candidate) {
	for {
		select {
		case <-ctx.Done():
			return
		case candidate, ok := <-events:
			if !ok {
				return
			}
			for _, chatID := range admins {
				message := renderFaceTouchCandidate(candidate)
				if err := h.rich.send(ctx, chatID, message, 0); err != nil {
					log.Printf("Send face-touch candidate %s to %d failed: %s", candidate.Record.ID, chatID, err)
				}
			}
		}
	}
}

func renderFaceTouchCandidate(candidate facetouch.Candidate) models.InputRichMessage {
	percent := int(math.Round(candidate.Record.Score * 100))
	caption := fmt.Sprintf("Pinch near chin\nScore: %d%%\nIs this a hair-plucking pose?", percent)
	return renderPhoto(bytes.NewReader(candidate.Photo), "chin-"+candidate.Record.ID+".jpg", caption,
		richCallbackButton("👍", faceTouchLabelAction, candidate.Record.ID+":"+string(facetouch.LabelWatch)),
		richCallbackButton("👎", faceTouchLabelAction, candidate.Record.ID+":"+string(facetouch.LabelIgnore)))
}

func renderLabeledFaceTouch(record facetouch.Record, photo []byte) models.InputRichMessage {
	percent := int(math.Round(record.Score * 100))
	caption := fmt.Sprintf("Pinch near chin\nScore: %d%%", percent)
	icon := "❌"
	if record.Label == facetouch.LabelWatch {
		icon = "✅"
	}
	message := renderPhoto(bytes.NewReader(photo), "chin-"+record.ID+".jpg", caption)
	message.Blocks = append(message.Blocks, models.InputRichBlock{
		Type: models.RichBlockTypeButtons,
		InputRichBlockButtons: &models.InputRichBlockButtons{
			Buttons: []models.RichMessageButton{{
				Text: richText(icon), Disabled: &models.DisabledButton{},
			}},
			Align: "right",
		},
	})
	return message
}

func parseFaceTouchLabel(data string) (string, facetouch.Label, bool) {
	id, labelText, ok := strings.Cut(data, ":")
	if !ok || id == "" {
		return "", "", false
	}
	label := facetouch.Label(labelText)
	if label != facetouch.LabelWatch && label != facetouch.LabelIgnore {
		return "", "", false
	}
	return id, label, true
}

func (h *handlerRegistry) labelFaceTouch(c *updateContext) error {
	if h.faceTouch == nil {
		return c.AnswerCallback("Chin-pinch storage unavailable", true)
	}
	id, label, ok := parseFaceTouchLabel(c.Data())
	if !ok {
		return c.AnswerCallback("Invalid label", true)
	}
	record, err := h.faceTouch.SetLabel(id, label, time.Now())
	if err != nil {
		return c.AnswerCallback("Could not save label", true)
	}
	photo, err := h.faceTouch.ReadPhoto(id)
	if err != nil {
		return c.AnswerCallback("Label saved, but the photo is unavailable", true)
	}
	answer := "Saved: other gesture"
	if label == facetouch.LabelWatch {
		answer = "Saved: hair-plucking pose"
	}
	if err := c.AnswerCallback(answer, false); err != nil {
		log.Printf("Answer face-touch label callback failed: %s", err)
	}
	return c.EditRichMessage(renderLabeledFaceTouch(record, photo))
}

func (h *handlerRegistry) sendFaceTouchStats(c *updateContext) error {
	if h.faceTouch == nil {
		return c.RespondRichMessage(renderNotice("Chin-pinch statistics", "Storage unavailable."))
	}
	summary, err := h.faceTouch.Summary()
	if err != nil {
		return c.RespondRichMessage(renderNotice("Chin-pinch statistics", err.Error()))
	}
	if summary.Total == 0 {
		text := "No pinch candidates yet."
		if summary.Legacy > 0 {
			text += fmt.Sprintf("\nPrevious hand-near-chin candidates preserved: %d", summary.Legacy)
		}
		return c.RespondRichMessage(renderNotice("Chin-pinch statistics", text))
	}
	latest := "—"
	if !summary.LatestAt.IsZero() {
		latest = summary.LatestAt.Format("02.01 15:04")
	}
	text := fmt.Sprintf("Candidates: %d\n👍 Track: %d\n👎 Ignore: %d\nUnlabeled: %d\nAccepted among labeled: %d%%\nDataset: %d photos, %d labeled\nTarget: 50–100 labeled photos per class\nLatest: %s",
		summary.Total, summary.Watch, summary.Ignore, summary.Pending, summary.AcceptedPercent(),
		summary.Dataset, summary.DatasetLabeled, latest)
	if summary.Legacy > 0 {
		text += fmt.Sprintf("\nPrevious hand-near-chin candidates: %d", summary.Legacy)
	}
	return c.RespondRichMessage(renderNotice("Chin-pinch statistics", text))
}
