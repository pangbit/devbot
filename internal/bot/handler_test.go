package bot

import (
    "context"
    "encoding/json"
    "testing"

    larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type fakeSender struct {
    called bool
    chatID string
    text   string
}

func (f *fakeSender) SendText(_ context.Context, chatID, text string) error {
    f.called = true
    f.chatID = chatID
    f.text = text
    return nil
}

func TestHandleMessage_TextEcho(t *testing.T) {
    raw := []byte(`{
        "schema":"2.0",
        "header":{"event_type":"im.message.receive_v1"},
        "event":{
            "sender":{"sender_type":"user"},
            "message":{
                "chat_id":"oc_test_chat",
                "message_type":"text",
                "content":"{\"text\":\"hello\"}"
            }
        }
    }`)

    var evt larkim.P2MessageReceiveV1
    if err := json.Unmarshal(raw, &evt); err != nil {
        t.Fatalf("unmarshal event: %v", err)
    }

    sender := &fakeSender{}
    h := NewHandler(sender, true)

    if err := h.HandleMessage(context.Background(), &evt); err != nil {
        t.Fatalf("HandleMessage error: %v", err)
    }

    if !sender.called {
        t.Fatalf("expected sender to be called")
    }
    if sender.chatID != "oc_test_chat" || sender.text != "hello" {
        t.Fatalf("unexpected send args: %q %q", sender.chatID, sender.text)
    }
}
