package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type fakeRouter struct {
	called    bool
	chatID    string
	userID    string
	text      string
	imageData []byte
	imageName string
	fileData  []byte
	fileName  string
	docID     string
}

func (f *fakeRouter) Route(_ context.Context, chatID, userID, text string) {
	f.called = true
	f.chatID = chatID
	f.userID = userID
	f.text = text
}

func (f *fakeRouter) RouteImage(_ context.Context, chatID, userID string, imageData []byte, fileName string) {
	f.called = true
	f.chatID = chatID
	f.userID = userID
	f.imageData = imageData
	f.imageName = fileName
}

func (f *fakeRouter) RouteFile(_ context.Context, chatID, userID, fileName string, fileData []byte) {
	f.called = true
	f.chatID = chatID
	f.userID = userID
	f.fileName = fileName
	f.fileData = fileData
}

func (f *fakeRouter) RouteDocShare(_ context.Context, chatID, userID, docID string) {
	f.called = true
	f.chatID = chatID
	f.userID = userID
	f.docID = docID
}

type fakeDownloader struct {
	imageData []byte
	imageErr  error
	fileData  []byte
	fileName  string
	fileErr   error
}

func (f *fakeDownloader) DownloadImage(_ context.Context, _, _ string) (io.ReadCloser, error) {
	if f.imageErr != nil {
		return nil, f.imageErr
	}
	return io.NopCloser(bytes.NewReader(f.imageData)), nil
}

func (f *fakeDownloader) DownloadFile(_ context.Context, _, _ string) (io.ReadCloser, string, error) {
	if f.fileErr != nil {
		return nil, "", f.fileErr
	}
	return io.NopCloser(bytes.NewReader(f.fileData)), f.fileName, nil
}

type fakeSender struct {
	messages []string
}

func (f *fakeSender) SendText(_ context.Context, _, text string) error {
	f.messages = append(f.messages, text)
	return nil
}

func (f *fakeSender) SendTextChunked(_ context.Context, _, text string) error {
	f.messages = append(f.messages, text)
	return nil
}

func makeEvent(senderType, senderID, chatID, chatType, msgType, content string, mentions []map[string]interface{}) []byte {
	return makeEventWithMsgID(senderType, senderID, chatID, chatType, msgType, content, "", mentions)
}

func makeEventWithMsgID(senderType, senderID, chatID, chatType, msgType, content, messageID string, mentions []map[string]interface{}) []byte {
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
				"message_id":   messageID,
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
	h := NewHandler(router, nil, nil, true, "bot_id")
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
	h := NewHandler(router, nil, nil, true, "bot_id")
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
	h := NewHandler(router, nil, nil, true, "bot_id")
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
	h := NewHandler(router, nil, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router called with @mention in group")
	}
	if router.text != "hello" {
		t.Fatalf("expected cleaned text 'hello', got %q", router.text)
	}
}

func TestHandleMessage_IgnoresUnsupportedType(t *testing.T) {
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "sticker", `{"sticker_key":"stk_xxx"}`, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for unsupported message type")
	}
}

func TestHandleMessage_ImageDownloadAndRoute(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`{"image_key":"img_abc123"}`, "msg_001", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	imgData := []byte("fake-png-data")
	dl := &fakeDownloader{imageData: imgData}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called for image message")
	}
	if router.chatID != "oc_chat" {
		t.Fatalf("chatID mismatch: %q", router.chatID)
	}
	if router.userID != "user1" {
		t.Fatalf("userID mismatch: %q", router.userID)
	}
	if !bytes.Equal(router.imageData, imgData) {
		t.Fatalf("imageData mismatch: got %v", router.imageData)
	}
	if router.imageName != "img_abc123.png" {
		t.Fatalf("imageName mismatch: %q", router.imageName)
	}
}

func TestHandleMessage_ImageDownloadError(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`{"image_key":"img_abc123"}`, "msg_001", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{imageErr: errors.New("network error")}
	sender := &fakeSender{}
	router := &fakeRouter{}
	h := NewHandler(router, dl, sender, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called when download fails")
	}
	if len(sender.messages) == 0 {
		t.Fatalf("expected error message sent to user")
	}
	if sender.messages[0] != "Failed to download image: network error" {
		t.Fatalf("unexpected error message: %q", sender.messages[0])
	}
}

func TestHandleMessage_ImageNoDownloader(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`{"image_key":"img_abc123"}`, "msg_001", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called when downloader is nil")
	}
}

func TestHandleMessage_FileDownloadAndRoute(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "file",
		`{"file_key":"file_xyz","file_name":"report.pdf"}`, "msg_002", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	fileData := []byte("fake-pdf-data")
	dl := &fakeDownloader{fileData: fileData, fileName: "server_report.pdf"}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called for file message")
	}
	if router.chatID != "oc_chat" {
		t.Fatalf("chatID mismatch: %q", router.chatID)
	}
	if router.userID != "user1" {
		t.Fatalf("userID mismatch: %q", router.userID)
	}
	if !bytes.Equal(router.fileData, fileData) {
		t.Fatalf("fileData mismatch: got %v", router.fileData)
	}
	// file_name from content should take priority over server-provided name
	if router.fileName != "report.pdf" {
		t.Fatalf("fileName mismatch: expected 'report.pdf', got %q", router.fileName)
	}
}

func TestHandleMessage_FileUsesServerName(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "file",
		`{"file_key":"file_xyz"}`, "msg_002", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{fileData: []byte("data"), fileName: "server_name.txt"}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called")
	}
	if router.fileName != "server_name.txt" {
		t.Fatalf("expected server name fallback, got %q", router.fileName)
	}
}

func TestHandleMessage_FileFallsBackToFileKey(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "file",
		`{"file_key":"file_xyz"}`, "msg_002", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{fileData: []byte("data"), fileName: ""}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called")
	}
	if router.fileName != "file_xyz" {
		t.Fatalf("expected file_key fallback, got %q", router.fileName)
	}
}

func TestHandleMessage_FileDownloadError(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "file",
		`{"file_key":"file_xyz","file_name":"report.pdf"}`, "msg_002", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{fileErr: errors.New("download failed")}
	sender := &fakeSender{}
	router := &fakeRouter{}
	h := NewHandler(router, dl, sender, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called when download fails")
	}
	if len(sender.messages) == 0 {
		t.Fatalf("expected error message sent to user")
	}
	if sender.messages[0] != "Failed to download file: download failed" {
		t.Fatalf("unexpected error message: %q", sender.messages[0])
	}
}

func TestHandleMessage_FileNoDownloader(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "file",
		`{"file_key":"file_xyz","file_name":"report.pdf"}`, "msg_002", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called when downloader is nil")
	}
}

func TestHandleMessage_DocURLInText(t *testing.T) {
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "text",
		`{"text":"https://abc.feishu.cn/docx/DOC123abc"}`, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected RouteDocShare to be called")
	}
	if router.docID != "DOC123abc" {
		t.Fatalf("expected docID 'DOC123abc', got %q", router.docID)
	}
}

func TestHandleMessage_DocURLInTextWithCommand(t *testing.T) {
	// A /doc bind command containing a URL should go through Route, not RouteDocShare
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "text",
		`{"text":"/doc bind readme https://abc.feishu.cn/docx/DOC123"}`, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected Route to be called")
	}
	if router.docID != "" {
		t.Fatalf("expected docID empty (routed as command), got %q", router.docID)
	}
	if router.text != "/doc bind readme https://abc.feishu.cn/docx/DOC123" {
		t.Fatalf("expected full text routed, got %q", router.text)
	}
}

func TestHandleMessage_PostWithDocURL(t *testing.T) {
	postContent := `{"content":[[{"tag":"a","href":"https://test.feishu.cn/docx/POST456","text":"My Document"}]]}`
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "post", postContent, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected RouteDocShare to be called for post with doc URL")
	}
	if router.docID != "POST456" {
		t.Fatalf("expected docID 'POST456', got %q", router.docID)
	}
}

func TestHandleMessage_PostWithoutDocURL(t *testing.T) {
	postContent := `{"content":[[{"tag":"text","text":"just some text"}]]}`
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "post", postContent, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id")
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for post without doc URL")
	}
}

func TestExtractDocID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://abc.feishu.cn/docx/ABC123", "ABC123"},
		{"https://feishu.cn/docx/XYZ789?query=1", "XYZ789"},
		{"check this https://test.feishu.cn/docx/DOC456 out", "DOC456"},
		{"no url here", ""},
		{"https://abc.feishu.cn/wiki/something", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractDocID(tt.input)
		if got != tt.want {
			t.Errorf("extractDocID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
