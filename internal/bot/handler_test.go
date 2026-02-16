package bot

import (
	"context"
	"encoding/json"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type fakeRouter struct {
	called bool
	chatID string
	userID string
	text   string
}

func (f *fakeRouter) Route(_ context.Context, chatID, userID, text string) {
	f.called = true
	f.chatID = chatID
	f.userID = userID
	f.text = text
}

func makeEvent(senderType, senderID, chatID, chatType, msgType, content string, mentions []map[string]interface{}) []byte {
	evt := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{"event_type": "im.message.receive_v1"},
		"event": map[string]interface{}{
			"sender": map[string]interface{}{
				"sender_type": senderType,
				"sender_id": map[string]interface{}{
					"open_id": senderID,
				},
			},
			"message": map[string]interface{}{
				"chat_id":      chatID,
				"chat_type":    chatType,
				"message_type": msgType,
				"content":      content,
				"mentions":     mentions,
			},
		},
	}
	data, _ := json.Marshal(evt)
	return data
}

func TestHandleMessage_TextToRouter(t *testing.T) {
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "text", `{"text":"hello"}`, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called")
	}
	if router.chatID != "oc_chat" {
		t.Fatalf("chatID mismatch: %q", router.chatID)
	}
	if router.userID != "user1" {
		t.Fatalf("userID mismatch: %q", router.userID)
	}
	if router.text != "hello" {
		t.Fatalf("text mismatch: %q", router.text)
	}
}

func TestHandleMessage_IgnoresBotSender(t *testing.T) {
	raw := makeEvent("app", "bot", "oc_chat", "p2p", "text", `{"text":"hello"}`, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for bot sender")
	}
}

func TestHandleMessage_GroupChatRequiresMention(t *testing.T) {
	raw := makeEvent("user", "user1", "oc_chat", "group", "text", `{"text":"hello"}`, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called without @mention in group")
	}
}

func TestHandleMessage_GroupChatWithMention(t *testing.T) {
	mentions := []map[string]interface{}{
		{
			"id":  map[string]interface{}{"open_id": "bot_id"},
			"key": "@_user_bot",
		},
	}
	raw := makeEvent("user", "user1", "oc_chat", "group", "text", `{"text":"@_user_bot hello"}`, mentions)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router called with @mention in group")
	}
	if router.text != "hello" {
		t.Fatalf("expected cleaned text 'hello', got %q", router.text)
	}
}

func TestHandleMessage_IgnoresNonText(t *testing.T) {
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "image", `{"image_key":"img_xxx"}`, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for non-text message")
	}
}
