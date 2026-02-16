package bot

import (
    "context"
    "encoding/json"
    "errors"

    larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type Sender interface {
	SendText(ctx context.Context, chatID, text string) error
	SendTextChunked(ctx context.Context, chatID, text string) error
}

type Handler struct {
    sender      Sender
    skipBotSelf bool
}

type eventEnvelope struct {
    Event struct {
        Sender struct {
            SenderType string `json:"sender_type"`
        } `json:"sender"`
        Message struct {
            ChatID      string `json:"chat_id"`
            MessageType string `json:"message_type"`
            Content     string `json:"content"`
        } `json:"message"`
    } `json:"event"`
}

type textContent struct {
    Text string `json:"text"`
}

func NewHandler(sender Sender, skipBotSelf bool) *Handler {
    return &Handler{sender: sender, skipBotSelf: skipBotSelf}
}

func (h *Handler) HandleMessage(ctx context.Context, evt *larkim.P2MessageReceiveV1) error {
    if h.sender == nil {
        return errors.New("sender is nil")
    }

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
    if env.Event.Message.MessageType != "text" {
        return nil
    }

    var content textContent
    if err := json.Unmarshal([]byte(env.Event.Message.Content), &content); err != nil {
        return nil
    }
    if content.Text == "" {
        return nil
    }

    return h.sender.SendText(ctx, env.Event.Message.ChatID, content.Text)
}
