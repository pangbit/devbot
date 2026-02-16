package bot

import (
	"context"
	"encoding/json"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type Sender interface {
	SendText(ctx context.Context, chatID, text string) error
	SendTextChunked(ctx context.Context, chatID, text string) error
}

type MessageRouter interface {
	Route(ctx context.Context, chatID, userID, text string)
}

type Handler struct {
	router      MessageRouter
	skipBotSelf bool
	botID       string
}

type eventEnvelope struct {
	Event struct {
		Sender struct {
			SenderType string `json:"sender_type"`
			SenderID   struct {
				OpenID string `json:"open_id"`
			} `json:"sender_id"`
		} `json:"sender"`
		Message struct {
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

func NewHandler(router MessageRouter, skipBotSelf bool, botID string) *Handler {
	return &Handler{router: router, skipBotSelf: skipBotSelf, botID: botID}
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

	if env.Event.Message.MessageType == "text" {
		var content textContent
		if err := json.Unmarshal([]byte(env.Event.Message.Content), &content); err != nil {
			return nil
		}
		text := h.cleanMentions(content.Text, env)
		if text == "" {
			return nil
		}
		h.router.Route(ctx, env.Event.Message.ChatID, env.Event.Sender.SenderID.OpenID, text)
		return nil
	}

	// TODO: handle image and file messages
	return nil
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
