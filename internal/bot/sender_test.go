package bot

import "testing"

func TestBuildSendMessageBody(t *testing.T) {
    body := buildSendMessageBody("oc_test_chat", "hello")
    if body["receive_id"] != "oc_test_chat" {
        t.Fatalf("receive_id mismatch")
    }
    if body["msg_type"] != "text" {
        t.Fatalf("msg_type mismatch")
    }
    if body["content"] == "" {
        t.Fatalf("content should not be empty")
    }
}
