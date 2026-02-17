package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type Sender interface {
	SendText(ctx context.Context, chatID, text string) error
	SendTextChunked(ctx context.Context, chatID, text string) error
	SendCard(ctx context.Context, chatID string, card CardMsg) error
}

type ImageAttachment struct {
	Data     []byte
	FileName string
}

type MessageRouter interface {
	Route(ctx context.Context, chatID, userID, text string)
	RouteImage(ctx context.Context, chatID, userID string, imageData []byte, fileName string)
	RouteFile(ctx context.Context, chatID, userID, fileName string, fileData []byte)
	RouteDocShare(ctx context.Context, chatID, userID, docID string)
	RouteTextWithImages(ctx context.Context, chatID, userID, text string, images []ImageAttachment)
}

// Downloader downloads images and files from Feishu.
type Downloader interface {
	// DownloadImage downloads an image resource from a message.
	// Uses the MessageResource API since Image.Get only works for bot-uploaded images.
	DownloadImage(ctx context.Context, messageID, imageKey string) (io.ReadCloser, error)
	// DownloadFile downloads a file resource from a message.
	// Returns a reader and the server-provided filename.
	DownloadFile(ctx context.Context, messageID, fileKey string) (io.ReadCloser, string, error)
}

const maxImageSize = 10 << 20 // 10 MB
const maxFileSize = 50 << 20  // 50 MB

var feishuDocURLPattern = regexp.MustCompile(`https?://[a-zA-Z0-9.-]*feishu\.cn/docx/([a-zA-Z0-9]+)`)

type Handler struct {
	router       MessageRouter
	downloader   Downloader
	sender       Sender
	skipBotSelf  bool
	botID        string
	allowedUsers map[string]bool
}

type eventEnvelope struct {
	Event struct {
		Sender struct {
			SenderType string `json:"sender_type"`
			SenderID   struct {
				OpenID string `json:"open_id"`
				UserID string `json:"user_id"`
			} `json:"sender_id"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			ChatID      string `json:"chat_id"`
			ChatType    string `json:"chat_type"`
			MessageType string `json:"message_type"`
			Content     string `json:"content"`
			Mentions    []struct {
				ID struct {
					OpenID string `json:"open_id"`
				} `json:"id"`
				Key string `json:"key"`
			} `json:"mentions"`
		} `json:"message"`
	} `json:"event"`
}

type textContent struct {
	Text string `json:"text"`
}

type imageContent struct {
	ImageKey string `json:"image_key"`
}

type fileContent struct {
	FileKey  string `json:"file_key"`
	FileName string `json:"file_name"`
}

// postContent represents the Feishu post (rich text) message structure.
type postContent struct {
	Title   string          `json:"title"`
	Content [][]postElement `json:"content"`
}

type postElement struct {
	Tag      string `json:"tag"`
	Text     string `json:"text,omitempty"`
	ImageKey string `json:"image_key,omitempty"`
	Href     string `json:"href,omitempty"`
}

func NewHandler(router MessageRouter, downloader Downloader, sender Sender, skipBotSelf bool, botID string, allowedUsers map[string]bool) *Handler {
	return &Handler{
		router:       router,
		downloader:   downloader,
		sender:       sender,
		skipBotSelf:  skipBotSelf,
		botID:        botID,
		allowedUsers: allowedUsers,
	}
}

func (h *Handler) HandleMessage(ctx context.Context, evt *larkim.P2MessageReceiveV1) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}

	var env eventEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}

	if h.skipBotSelf && env.Event.Sender.SenderType != "user" {
		return nil
	}

	if env.Event.Message.ChatType == "group" {
		if !h.isMentioned(env) {
			return nil
		}
	}

	chatID := env.Event.Message.ChatID
	userID := h.resolveUserID(env)
	messageID := env.Event.Message.MessageID

	log.Printf("handler: received %s from user=%s chat=%s", env.Event.Message.MessageType, userID, chatID)

	switch env.Event.Message.MessageType {
	case "text":
		var content textContent
		if err := json.Unmarshal([]byte(env.Event.Message.Content), &content); err != nil {
			return nil
		}
		text := h.cleanMentions(content.Text, env)
		if text == "" {
			return nil
		}
		// Detect feishu doc URL shared as plain text
		if docID := extractDocID(text); docID != "" && !strings.HasPrefix(text, "/") {
			h.router.RouteDocShare(ctx, chatID, userID, docID)
			return nil
		}
		h.router.Route(ctx, chatID, userID, text)

	case "post":
		// Rich text messages — extract doc URL if present
		if docID := extractDocID(env.Event.Message.Content); docID != "" {
			h.router.RouteDocShare(ctx, chatID, userID, docID)
		} else {
			h.handlePost(ctx, chatID, userID, messageID, env)
		}

	case "interactive":
		// Interactive cards — extract doc URL if present
		if docID := extractDocID(env.Event.Message.Content); docID != "" {
			h.router.RouteDocShare(ctx, chatID, userID, docID)
		}

	case "image":
		h.handleImage(ctx, chatID, userID, messageID, env.Event.Message.Content)

	case "file":
		h.handleFile(ctx, chatID, userID, messageID, env.Event.Message.Content)
	}

	return nil
}

func (h *Handler) handleImage(ctx context.Context, chatID, userID, messageID, rawContent string) {
	var content imageContent
	if err := json.Unmarshal([]byte(rawContent), &content); err != nil {
		log.Printf("handler: failed to parse image content: %v", err)
		return
	}
	if content.ImageKey == "" {
		return
	}
	if h.downloader == nil {
		log.Println("handler: downloader not configured, cannot download image")
		return
	}

	reader, err := h.downloader.DownloadImage(ctx, messageID, content.ImageKey)
	if err != nil {
		log.Printf("handler: failed to download image %s: %v", content.ImageKey, err)
		if h.sender != nil {
			h.sender.SendText(ctx, chatID, fmt.Sprintf("Failed to download image: %v", err))
		}
		return
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxImageSize+1))
	if err != nil {
		log.Printf("handler: failed to read image data: %v", err)
		return
	}
	if len(data) > maxImageSize {
		log.Printf("handler: image %s exceeds max size (%d bytes)", content.ImageKey, maxImageSize)
		if h.sender != nil {
			h.sender.SendText(ctx, chatID, fmt.Sprintf("Image too large (max %d MB)", maxImageSize>>20))
		}
		return
	}

	ext := ".png"
	switch ct := http.DetectContentType(data); {
	case strings.HasPrefix(ct, "image/jpeg"):
		ext = ".jpg"
	case strings.HasPrefix(ct, "image/gif"):
		ext = ".gif"
	case strings.HasPrefix(ct, "image/webp"):
		ext = ".webp"
	}
	h.router.RouteImage(ctx, chatID, userID, data, content.ImageKey+ext)
}

func (h *Handler) handleFile(ctx context.Context, chatID, userID, messageID, rawContent string) {
	var content fileContent
	if err := json.Unmarshal([]byte(rawContent), &content); err != nil {
		log.Printf("handler: failed to parse file content: %v", err)
		return
	}
	if content.FileKey == "" {
		return
	}
	if h.downloader == nil {
		log.Println("handler: downloader not configured, cannot download file")
		return
	}

	reader, serverName, err := h.downloader.DownloadFile(ctx, messageID, content.FileKey)
	if err != nil {
		log.Printf("handler: failed to download file %s: %v", content.FileKey, err)
		if h.sender != nil {
			h.sender.SendText(ctx, chatID, fmt.Sprintf("Failed to download file: %v", err))
		}
		return
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxFileSize+1))
	if err != nil {
		log.Printf("handler: failed to read file data: %v", err)
		return
	}
	if len(data) > maxFileSize {
		log.Printf("handler: file %s exceeds max size (%d bytes)", content.FileKey, maxFileSize)
		if h.sender != nil {
			h.sender.SendText(ctx, chatID, fmt.Sprintf("File too large (max %d MB)", maxFileSize>>20))
		}
		return
	}

	// Prefer the filename from the message content, fallback to server-provided name
	fileName := content.FileName
	if fileName == "" {
		fileName = serverName
	}
	if fileName == "" {
		fileName = content.FileKey
	}

	h.router.RouteFile(ctx, chatID, userID, fileName, data)
}

// resolveUserID returns the sender ID that matches the allowedUsers list.
// Supports both open_id (ou_xxx) and user_id formats in DEVBOT_ALLOWED_USER_IDS.
func (h *Handler) resolveUserID(env eventEnvelope) string {
	openID := env.Event.Sender.SenderID.OpenID
	if h.allowedUsers[openID] {
		return openID
	}
	if uid := env.Event.Sender.SenderID.UserID; uid != "" && h.allowedUsers[uid] {
		return uid
	}
	return openID // fallback to open_id (router will log unauthorized)
}

func (h *Handler) isMentioned(env eventEnvelope) bool {
	for _, m := range env.Event.Message.Mentions {
		if m.ID.OpenID == h.botID {
			return true
		}
	}
	return false
}

func (h *Handler) cleanMentions(text string, env eventEnvelope) string {
	for _, m := range env.Event.Message.Mentions {
		text = strings.ReplaceAll(text, m.Key, "")
	}
	return strings.TrimSpace(text)
}

// handlePost extracts text and images from a Feishu post (rich text) message.
func (h *Handler) handlePost(ctx context.Context, chatID, userID, messageID string, env eventEnvelope) {
	var pc postContent
	if err := json.Unmarshal([]byte(env.Event.Message.Content), &pc); err != nil {
		log.Printf("handler: failed to parse post content: %v", err)
		return
	}

	// Collect text and image keys from all paragraphs
	var texts []string
	var imageKeys []string
	for _, para := range pc.Content {
		for _, elem := range para {
			switch elem.Tag {
			case "text":
				if t := strings.TrimSpace(elem.Text); t != "" {
					texts = append(texts, t)
				}
			case "img":
				if elem.ImageKey != "" {
					imageKeys = append(imageKeys, elem.ImageKey)
				}
			case "a":
				if t := strings.TrimSpace(elem.Text); t != "" {
					texts = append(texts, t)
				}
			}
		}
	}
	if pc.Title != "" {
		texts = append([]string{pc.Title}, texts...)
	}

	text := h.cleanMentions(strings.Join(texts, "\n"), env)

	// Download images
	var images []ImageAttachment
	for _, key := range imageKeys {
		if att := h.downloadPostImageData(ctx, chatID, messageID, key); att != nil {
			images = append(images, *att)
		}
	}

	if text == "" && len(images) == 0 {
		return
	}

	if len(images) == 0 {
		// Text-only post, route as plain text
		h.router.Route(ctx, chatID, userID, text)
	} else {
		// Has images (with or without text)
		h.router.RouteTextWithImages(ctx, chatID, userID, text, images)
	}
}

// downloadPostImageData downloads an image from a post message and returns the data.
func (h *Handler) downloadPostImageData(ctx context.Context, chatID, messageID, imageKey string) *ImageAttachment {
	if h.downloader == nil {
		log.Println("handler: downloader not configured, cannot download image")
		return nil
	}

	reader, err := h.downloader.DownloadImage(ctx, messageID, imageKey)
	if err != nil {
		log.Printf("handler: failed to download post image %s: %v", imageKey, err)
		return nil
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxImageSize+1))
	if err != nil {
		log.Printf("handler: failed to read post image data: %v", err)
		return nil
	}
	if len(data) > maxImageSize {
		log.Printf("handler: post image %s exceeds max size", imageKey)
		return nil
	}

	ext := ".png"
	switch ct := http.DetectContentType(data); {
	case strings.HasPrefix(ct, "image/jpeg"):
		ext = ".jpg"
	case strings.HasPrefix(ct, "image/gif"):
		ext = ".gif"
	case strings.HasPrefix(ct, "image/webp"):
		ext = ".webp"
	}

	return &ImageAttachment{Data: data, FileName: imageKey + ext}
}

// extractDocID finds the first Feishu doc URL in text and returns the document ID.
func extractDocID(text string) string {
	m := feishuDocURLPattern.FindStringSubmatch(text)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}
