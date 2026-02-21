package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
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
	images    []ImageAttachment
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

func (f *fakeRouter) RouteTextWithImages(_ context.Context, chatID, userID, text string, images []ImageAttachment) {
	f.called = true
	f.chatID = chatID
	f.userID = userID
	f.text = text
	f.images = images
}

// errReadCloser is an io.ReadCloser whose Read always fails.
type errReadCloser struct{}

func (e *errReadCloser) Read(p []byte) (int, error) { return 0, errors.New("read error") }
func (e *errReadCloser) Close() error               { return nil }

// errReadDownloader always returns a reader that fails on Read.
type errReadDownloader struct{}

func (e *errReadDownloader) DownloadImage(_ context.Context, _, _ string) (io.ReadCloser, error) {
	return &errReadCloser{}, nil
}
func (e *errReadDownloader) DownloadFile(_ context.Context, _, _ string) (io.ReadCloser, string, error) {
	return &errReadCloser{}, "", nil
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

func (f *fakeSender) SendCard(_ context.Context, _ string, card CardMsg) error {
	f.messages = append(f.messages, card.Title+"\n\n"+card.Content)
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
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
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
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
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
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
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
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
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
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
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
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
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
	h := NewHandler(router, dl, sender, true, "bot_id", nil)
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
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
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
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
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
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
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
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
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
	h := NewHandler(router, dl, sender, true, "bot_id", nil)
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
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called when downloader is nil")
	}
}

func TestHandleMessage_ImageInvalidContent(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`not valid json {{{`, "msg_050", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for invalid image content")
	}
}

func TestHandleMessage_ImageEmptyKey(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`{"image_key":""}`, "msg_051", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for empty image key")
	}
}

func TestHandleMessage_ImageDownloadError_NoSender(t *testing.T) {
	// When download fails and h.sender is nil, should not panic.
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`{"image_key":"img_abc"}`, "msg_052", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{imageErr: errors.New("network error")}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil) // sender = nil
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called when download fails")
	}
}

func TestHandleMessage_FileInvalidContent(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "file",
		`not valid json {{{`, "msg_053", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for invalid file content")
	}
}

func TestHandleMessage_FileEmptyKey(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "file",
		`{"file_key":"","file_name":"test.txt"}`, "msg_054", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for empty file key")
	}
}

func TestHandleMessage_InteractiveWithDocURL(t *testing.T) {
	evt := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{"event_type": "im.message.receive_v1"},
		"event": map[string]interface{}{
			"sender": map[string]interface{}{
				"sender_type": "user",
				"sender_id":   map[string]interface{}{"open_id": "user1"},
			},
			"message": map[string]interface{}{
				"chat_id":      "oc_chat",
				"chat_type":    "p2p",
				"message_type": "interactive",
				"content":      `{"url":"https://abc.feishu.cn/docx/INTERACT456"}`,
			},
		},
	}
	data, _ := json.Marshal(evt)
	var parsed larkim.P2MessageReceiveV1
	json.Unmarshal(data, &parsed)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &parsed)

	if !router.called {
		t.Fatalf("expected RouteDocShare called for interactive with doc URL")
	}
	if router.docID != "INTERACT456" {
		t.Fatalf("expected docID 'INTERACT456', got %q", router.docID)
	}
}

func TestHandleMessage_DocURLInText(t *testing.T) {
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "text",
		`{"text":"https://abc.feishu.cn/docx/DOC123abc"}`, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
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
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
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
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected RouteDocShare to be called for post with doc URL")
	}
	if router.docID != "POST456" {
		t.Fatalf("expected docID 'POST456', got %q", router.docID)
	}
}

func TestHandleMessage_PostWithText(t *testing.T) {
	postContent := `{"content":[[{"tag":"text","text":"just some text"}]]}`
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "post", postContent, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called for post with text")
	}
	if router.text != "just some text" {
		t.Fatalf("expected text 'just some text', got %q", router.text)
	}
}

func TestHandleMessage_ImageTooLarge(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`{"image_key":"img_abc123"}`, "msg_001", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	// Image exceeds 10MB limit
	largeData := make([]byte, maxImageSize+1)
	dl := &fakeDownloader{imageData: largeData}
	sender := &fakeSender{}
	router := &fakeRouter{}
	h := NewHandler(router, dl, sender, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for oversized image")
	}
	if len(sender.messages) == 0 || !strings.Contains(sender.messages[0], "too large") {
		t.Fatalf("expected 'too large' message, got: %v", sender.messages)
	}
}

func TestHandleMessage_FileTooLarge(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "file",
		`{"file_key":"file_xyz","file_name":"report.pdf"}`, "msg_002", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	// File exceeds 50MB limit
	largeData := make([]byte, maxFileSize+1)
	dl := &fakeDownloader{fileData: largeData}
	sender := &fakeSender{}
	router := &fakeRouter{}
	h := NewHandler(router, dl, sender, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for oversized file")
	}
	if len(sender.messages) == 0 || !strings.Contains(sender.messages[0], "too large") {
		t.Fatalf("expected 'too large' message, got: %v", sender.messages)
	}
}

func TestHandleMessage_ImageJPEGExtension(t *testing.T) {
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`{"image_key":"img_jpeg"}`, "msg_003", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	// JPEG magic bytes: ff d8 ff
	jpegData := append([]byte{0xff, 0xd8, 0xff, 0xe0}, make([]byte, 100)...)
	dl := &fakeDownloader{imageData: jpegData}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called")
	}
	if router.imageName != "img_jpeg.jpg" {
		t.Fatalf("expected .jpg extension, got: %q", router.imageName)
	}
}

func TestHandleMessage_ResolvesUserIDFormat(t *testing.T) {
	// Event has both open_id and user_id
	evt := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{"event_type": "im.message.receive_v1"},
		"event": map[string]interface{}{
			"sender": map[string]interface{}{
				"sender_type": "user",
				"sender_id": map[string]interface{}{
					"open_id": "ou_abc123",
					"user_id": "testuser1",
				},
			},
			"message": map[string]interface{}{
				"chat_id":      "oc_chat",
				"chat_type":    "p2p",
				"message_type": "text",
				"content":      `{"text":"hello"}`,
			},
		},
	}
	data, _ := json.Marshal(evt)
	var parsed larkim.P2MessageReceiveV1
	json.Unmarshal(data, &parsed)

	// Allowlist uses user_id format instead of open_id
	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", map[string]bool{"testuser1": true})
	h.HandleMessage(context.Background(), &parsed)

	if !router.called {
		t.Fatalf("expected router to be called")
	}
	if router.userID != "testuser1" {
		t.Fatalf("expected resolved user_id 'testuser1', got: %q", router.userID)
	}
}

func TestHandleMessage_PrefersOpenID(t *testing.T) {
	evt := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{"event_type": "im.message.receive_v1"},
		"event": map[string]interface{}{
			"sender": map[string]interface{}{
				"sender_type": "user",
				"sender_id": map[string]interface{}{
					"open_id": "ou_abc123",
					"user_id": "testuser1",
				},
			},
			"message": map[string]interface{}{
				"chat_id":      "oc_chat",
				"chat_type":    "p2p",
				"message_type": "text",
				"content":      `{"text":"hello"}`,
			},
		},
	}
	data, _ := json.Marshal(evt)
	var parsed larkim.P2MessageReceiveV1
	json.Unmarshal(data, &parsed)

	// Allowlist uses open_id — should prefer it over user_id
	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", map[string]bool{"ou_abc123": true})
	h.HandleMessage(context.Background(), &parsed)

	if !router.called {
		t.Fatalf("expected router to be called")
	}
	if router.userID != "ou_abc123" {
		t.Fatalf("expected open_id 'ou_abc123', got: %q", router.userID)
	}
}

func TestHandleMessage_PostWithTextAndImage(t *testing.T) {
	postContent := `{"content":[[{"tag":"text","text":"fix this bug"}],[{"tag":"img","image_key":"img_abc123"}]]}`
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "post", postContent, "msg_010", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	imgData := []byte("fake-png-data")
	dl := &fakeDownloader{imageData: imgData}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called for post with text+image")
	}
	if router.text != "fix this bug" {
		t.Fatalf("expected text 'fix this bug', got %q", router.text)
	}
	if len(router.images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(router.images))
	}
	if !bytes.Equal(router.images[0].Data, imgData) {
		t.Fatalf("image data mismatch")
	}
}

func TestHandleMessage_PostWithImageOnly(t *testing.T) {
	postContent := `{"content":[[{"tag":"img","image_key":"img_xyz"}]]}`
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "post", postContent, "msg_011", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	imgData := []byte("fake-png-data")
	dl := &fakeDownloader{imageData: imgData}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called for post with image")
	}
	if router.text != "" {
		t.Fatalf("expected empty text, got %q", router.text)
	}
	if len(router.images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(router.images))
	}
}

func TestHandleMessage_PostWithImageNoDownloader(t *testing.T) {
	postContent := `{"content":[[{"tag":"text","text":"check this"}],[{"tag":"img","image_key":"img_abc"}]]}`
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "post", postContent, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	// Should still route the text, just without images
	if !router.called {
		t.Fatalf("expected router to be called with text even without downloader")
	}
	if router.text != "check this" {
		t.Fatalf("expected text 'check this', got %q", router.text)
	}
}

func TestHandleMessage_PostImageDownloadError(t *testing.T) {
	// When a post message has an image but the downloader fails, the image is
	// silently skipped and the text is still routed.
	postContent := `{"content":[[{"tag":"text","text":"fix this"},{"tag":"img","image_key":"img_bad"}]]}`
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "post", postContent, "msg_020", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{imageErr: errors.New("network error")}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	// Text should still be routed even though image download failed
	if !router.called {
		t.Fatalf("expected router to be called with text even when image download fails")
	}
	if router.text != "fix this" {
		t.Fatalf("expected text 'fix this', got %q", router.text)
	}
	// No images should have been attached
	if len(router.images) != 0 {
		t.Fatalf("expected no images (download failed), got %d", len(router.images))
	}
}

func TestHandleMessage_PostImageTooLarge(t *testing.T) {
	// When a post message has an oversized image, it is silently skipped.
	postContent := `{"content":[[{"tag":"text","text":"check this"},{"tag":"img","image_key":"img_huge"}]]}`
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "post", postContent, "msg_021", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	largeData := make([]byte, maxImageSize+1)
	dl := &fakeDownloader{imageData: largeData}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	// Text should still be routed even though the image was too large
	if !router.called {
		t.Fatalf("expected router to be called with text even when image is too large")
	}
	if router.text != "check this" {
		t.Fatalf("expected text 'check this', got %q", router.text)
	}
	// No images should have been attached
	if len(router.images) != 0 {
		t.Fatalf("expected no images (too large), got %d", len(router.images))
	}
}

func TestHandleMessage_ImageReadError(t *testing.T) {
	// DownloadImage succeeds but the reader returns an error during Read.
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`{"image_key":"img_err"}`, "msg_030", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &errReadDownloader{}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	// ReadAll error → silently skipped, router not called
	if router.called {
		t.Fatalf("expected router not called when read fails")
	}
}

func TestHandleMessage_PostImageReadError(t *testing.T) {
	// DownloadImage succeeds but Read fails (post image context).
	postContent := `{"content":[[{"tag":"text","text":"fix this"},{"tag":"img","image_key":"img_err2"}]]}`
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "post", postContent, "msg_041", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &errReadDownloader{}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	// Text still routed; image silently dropped
	if !router.called {
		t.Fatalf("expected router to be called with text even when image read fails")
	}
	if router.text != "fix this" {
		t.Fatalf("expected text 'fix this', got %q", router.text)
	}
	if len(router.images) != 0 {
		t.Fatalf("expected no images (read failed), got %d", len(router.images))
	}
}

func TestHandleMessage_PostWithTitle(t *testing.T) {
	postContent := `{"title":"Bug Report","content":[[{"tag":"text","text":"details here"}]]}`
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "post", postContent, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called")
	}
	if router.text != "Bug Report\ndetails here" {
		t.Fatalf("expected title+text, got %q", router.text)
	}
}

func TestHandleMessage_TextEmpty(t *testing.T) {
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "text", `{"text":""}`, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for empty text")
	}
}

func TestHandleMessage_TextInvalidContent(t *testing.T) {
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "text", `not valid json`, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called for invalid text content JSON")
	}
}

func TestHandleMessage_ImageGIF(t *testing.T) {
	// GIF89a magic bytes
	gifData := append([]byte("GIF89a"), make([]byte, 20)...)
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`{"image_key":"img_gif"}`, "msg_gif", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{imageData: gifData}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called for GIF image")
	}
	if router.imageName != "img_gif.gif" {
		t.Fatalf("expected .gif extension, got %q", router.imageName)
	}
}

func TestHandleMessage_ImageWEBP(t *testing.T) {
	// WEBP magic: RIFF + 4 bytes (any) + WEBP + VP (chunk type prefix)
	webpData := []byte{'R', 'I', 'F', 'F', 0x10, 0x20, 0x30, 0x40, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}
	webpData = append(webpData, make([]byte, 20)...)
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "image",
		`{"image_key":"img_webp"}`, "msg_webp", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{imageData: webpData}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called for WEBP image")
	}
	if router.imageName != "img_webp.webp" {
		t.Fatalf("expected .webp extension, got %q", router.imageName)
	}
}

func TestHandleMessage_FileReadError(t *testing.T) {
	// DownloadFile succeeds but Read fails — router should not be called.
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "file",
		`{"file_key":"file_xyz","file_name":"report.pdf"}`, "msg_read_err", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &errReadDownloader{}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called when file read fails")
	}
}

func TestHandleMessage_PostLinkTag(t *testing.T) {
	// Post with an "a" (hyperlink) tag — text should be collected and routed.
	postContent := `{"content":[[{"tag":"a","href":"https://example.com","text":"Click here"}]]}`
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "post", postContent, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called with link text")
	}
	if router.text != "Click here" {
		t.Fatalf("expected 'Click here', got %q", router.text)
	}
}

func TestHandleMessage_PostNoResult(t *testing.T) {
	// Post with only whitespace text and no images — should not call router.
	postContent := `{"content":[[{"tag":"text","text":"   "}]]}`
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "post", postContent, nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called when post has no meaningful content")
	}
}

func TestHandleMessage_PostImageJPEG(t *testing.T) {
	// JPEG magic bytes — downloadPostImageData should produce .jpg extension.
	jpegData := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01}
	postContent := `{"content":[[{"tag":"img","image_key":"img_jpeg"}]]}`
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "post", postContent, "msg_jpeg", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{imageData: jpegData}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called for JPEG post image")
	}
	if len(router.images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(router.images))
	}
	if !strings.HasSuffix(router.images[0].FileName, ".jpg") {
		t.Fatalf("expected .jpg extension, got %q", router.images[0].FileName)
	}
}

func TestHandleMessage_PostImageGIF(t *testing.T) {
	// GIF in post — downloadPostImageData should produce .gif extension.
	gifData := append([]byte("GIF89a"), make([]byte, 20)...)
	postContent := `{"content":[[{"tag":"img","image_key":"img_pgif"}]]}`
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "post", postContent, "msg_pgif", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{imageData: gifData}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called for GIF post image")
	}
	if len(router.images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(router.images))
	}
	if !strings.HasSuffix(router.images[0].FileName, ".gif") {
		t.Fatalf("expected .gif extension, got %q", router.images[0].FileName)
	}
}

func TestHandleMessage_PostInvalidJSON(t *testing.T) {
	// Invalid JSON in post content — handlePost should return early without calling router.
	raw := makeEvent("user", "user1", "oc_chat", "p2p", "post", "not valid json", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if router.called {
		t.Fatalf("expected router not called when post content is invalid JSON")
	}
}

func TestHandleMessage_PostImageWEBP(t *testing.T) {
	// WEBP in post — downloadPostImageData should produce .webp extension.
	webpData := []byte{'R', 'I', 'F', 'F', 0x10, 0x20, 0x30, 0x40, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}
	webpData = append(webpData, make([]byte, 20)...)
	postContent := `{"content":[[{"tag":"img","image_key":"img_pwebp"}]]}`
	raw := makeEventWithMsgID("user", "user1", "oc_chat", "p2p", "post", postContent, "msg_pwebp", nil)
	var evt larkim.P2MessageReceiveV1
	json.Unmarshal(raw, &evt)

	dl := &fakeDownloader{imageData: webpData}
	router := &fakeRouter{}
	h := NewHandler(router, dl, nil, true, "bot_id", nil)
	h.HandleMessage(context.Background(), &evt)

	if !router.called {
		t.Fatalf("expected router to be called for WEBP post image")
	}
	if len(router.images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(router.images))
	}
	if !strings.HasSuffix(router.images[0].FileName, ".webp") {
		t.Fatalf("expected .webp extension, got %q", router.images[0].FileName)
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
