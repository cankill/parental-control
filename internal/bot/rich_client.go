package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
)

const telegramAPIURL = "https://api.telegram.org"

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type richClient struct {
	baseURL string
	token   string
	client  httpDoer
}

func newRichClient(baseURL, token string, client httpDoer) *richClient {
	if client == nil {
		client = &http.Client{Timeout: time.Minute}
	}
	return &richClient{baseURL: strings.TrimRight(baseURL, "/"), token: token, client: client}
}

func (c *richClient) send(ctx context.Context, chatID int64, message models.InputRichMessage, replyTo int) error {
	fields := map[string]string{"chat_id": strconv.FormatInt(chatID, 10)}
	if replyTo != 0 {
		reply, err := json.Marshal(models.ReplyParameters{MessageID: replyTo})
		if err != nil {
			return fmt.Errorf("marshal reply parameters: %w", err)
		}
		fields["reply_parameters"] = string(reply)
	}
	return c.call(ctx, "sendRichMessage", fields, message)
}

func (c *richClient) edit(ctx context.Context, chatID int64, messageID int, message models.InputRichMessage) error {
	return c.call(ctx, "editMessageText", map[string]string{
		"chat_id":    strconv.FormatInt(chatID, 10),
		"message_id": strconv.Itoa(messageID),
	}, message)
}

func (c *richClient) call(ctx context.Context, method string, fields map[string]string, message models.InputRichMessage) error {
	encoded, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal rich message: %w", err)
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := form.WriteField(name, value); err != nil {
			return fmt.Errorf("write %s field: %w", name, err)
		}
	}
	if err := form.WriteField("rich_message", string(encoded)); err != nil {
		return fmt.Errorf("write rich_message field: %w", err)
	}
	for _, attachment := range richAttachments(message) {
		part, err := form.CreateFormFile(attachment.name, attachment.name)
		if err != nil {
			return fmt.Errorf("create %s attachment: %w", attachment.name, err)
		}
		if _, err := io.Copy(part, attachment.data); err != nil {
			return fmt.Errorf("write %s attachment: %w", attachment.name, err)
		}
	}
	if err := form.Close(); err != nil {
		return fmt.Errorf("close multipart request: %w", err)
	}

	url := c.baseURL + "/bot" + c.token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return fmt.Errorf("create Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("Telegram %s request: %w", method, err)
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		ErrorCode   int    `json:"error_code"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return fmt.Errorf("decode Telegram %s response (HTTP %d): %w", method, resp.StatusCode, err)
	}
	if !result.OK {
		return fmt.Errorf("Telegram %s failed (%d): %s", method, result.ErrorCode, result.Description)
	}
	return nil
}

type richAttachment struct {
	name string
	data io.Reader
}

type attachedMedia interface {
	GetMedia() string
	Attachment() io.Reader
}

func richAttachments(message models.InputRichMessage) []richAttachment {
	var attachments []richAttachment
	for _, media := range message.Media {
		if attached, ok := media.Media.(attachedMedia); ok {
			attachments = appendRichAttachment(attachments, attached)
		}
	}
	for i := range message.Blocks {
		attachments = appendBlockAttachments(attachments, &message.Blocks[i])
	}
	return attachments
}

func appendBlockAttachments(attachments []richAttachment, block *models.InputRichBlock) []richAttachment {
	if block == nil {
		return attachments
	}
	switch block.Type {
	case models.RichBlockTypePhoto:
		if block.InputRichBlockPhoto != nil {
			attachments = appendRichAttachment(attachments, &block.InputRichBlockPhoto.Photo)
		}
	case models.RichBlockTypeAudio:
		if block.InputRichBlockAudio != nil {
			attachments = appendRichAttachment(attachments, &block.InputRichBlockAudio.Audio)
		}
	case models.RichBlockTypeDocument:
		if block.InputRichBlockDocument != nil {
			attachments = appendRichAttachment(attachments, &block.InputRichBlockDocument.Document)
		}
	case models.RichBlockTypeVideo:
		if block.InputRichBlockVideo != nil {
			attachments = appendRichAttachment(attachments, &block.InputRichBlockVideo.Video)
		}
	case models.RichBlockTypeAnimation:
		if block.InputRichBlockAnimation != nil {
			attachments = appendRichAttachment(attachments, &block.InputRichBlockAnimation.Animation)
		}
	case models.RichBlockTypeVoiceNote:
		if block.InputRichBlockVoiceNote != nil {
			attachments = appendRichAttachment(attachments, &block.InputRichBlockVoiceNote.VoiceNote)
		}
	case models.RichBlockTypeCollage:
		if block.InputRichBlockCollage != nil {
			for i := range block.InputRichBlockCollage.Blocks {
				attachments = appendBlockAttachments(attachments, &block.InputRichBlockCollage.Blocks[i])
			}
		}
	case models.RichBlockTypeSlideshow:
		if block.InputRichBlockSlideshow != nil {
			for i := range block.InputRichBlockSlideshow.Blocks {
				attachments = appendBlockAttachments(attachments, &block.InputRichBlockSlideshow.Blocks[i])
			}
		}
	case models.RichBlockTypeDetails:
		if block.InputRichBlockDetails != nil {
			for i := range block.InputRichBlockDetails.Blocks {
				attachments = appendBlockAttachments(attachments, &block.InputRichBlockDetails.Blocks[i])
			}
		}
	case models.RichBlockTypeBlockQuotation:
		if block.InputRichBlockBlockQuotation != nil {
			for i := range block.InputRichBlockBlockQuotation.Blocks {
				attachments = appendBlockAttachments(attachments, &block.InputRichBlockBlockQuotation.Blocks[i])
			}
		}
	case models.RichBlockTypeList:
		if block.InputRichBlockList != nil {
			for i := range block.InputRichBlockList.Items {
				for j := range block.InputRichBlockList.Items[i].Blocks {
					attachments = appendBlockAttachments(attachments, &block.InputRichBlockList.Items[i].Blocks[j])
				}
			}
		}
	}
	return attachments
}

func appendRichAttachment(attachments []richAttachment, media attachedMedia) []richAttachment {
	name := strings.TrimPrefix(media.GetMedia(), "attach://")
	if name == media.GetMedia() || name == "" || media.Attachment() == nil {
		return attachments
	}
	return append(attachments, richAttachment{name: name, data: media.Attachment()})
}
