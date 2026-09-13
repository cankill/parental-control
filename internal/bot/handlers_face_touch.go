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
	caption := fmt.Sprintf("Hand near chin\nScore: %d%%\nIs this a case to track?", percent)
	return renderPhoto(bytes.NewReader(candidate.Photo), "chin-"+candidate.Record.ID+".jpg", caption,
		richCallbackButton("👍", faceTouchLabelAction, candidate.Record.ID+":"+string(facetouch.LabelWatch)),
		richCallbackButton("👎", faceTouchLabelAction, candidate.Record.ID+":"+string(facetouch.LabelIgnore)))
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
		return c.AnswerCallback("Chin-touch storage unavailable", true)
	}
	id, label, ok := parseFaceTouchLabel(c.Data())
	if !ok {
		return c.AnswerCallback("Invalid label", true)
	}
	if _, err := h.faceTouch.SetLabel(id, label, time.Now()); err != nil {
		return c.AnswerCallback("Could not save label", true)
	}
	if label == facetouch.LabelWatch {
		return c.AnswerCallback("Saved: track this", false)
	}
	return c.AnswerCallback("Saved: ignore this", false)
}

func (h *handlerRegistry) sendFaceTouchStats(c *updateContext) error {
	if h.faceTouch == nil {
		return c.RespondRichMessage(renderNotice("Chin-touch statistics", "Storage unavailable."))
	}
	summary, err := h.faceTouch.Summary()
	if err != nil {
		return c.RespondRichMessage(renderNotice("Chin-touch statistics", err.Error()))
	}
	if summary.Total == 0 {
		return c.RespondRichMessage(renderNotice("Chin-touch statistics", "No candidates yet."))
	}
	latest := "—"
	if !summary.LatestAt.IsZero() {
		latest = summary.LatestAt.Format("02.01 15:04")
	}
	text := fmt.Sprintf("Candidates: %d\n👍 Track: %d\n👎 Ignore: %d\nUnlabeled: %d\nAccepted among labeled: %d%%\nLatest: %s",
		summary.Total, summary.Watch, summary.Ignore, summary.Pending, summary.AcceptedPercent(), latest)
	return c.RespondRichMessage(renderNotice("Chin-touch statistics", text))
}
