package bot

import (
	"testing"
	"unicode/utf8"
)

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

func TestSplitMessage_Short(t *testing.T) {
	chunks := SplitMessage("hello world", 4000)
	if len(chunks) != 1 || chunks[0] != "hello world" {
		t.Fatalf("expected single chunk, got %d: %v", len(chunks), chunks)
	}
}

func TestSplitMessage_Long(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "line " + string(rune('A'+i%26)) + "\n"
	}
	chunks := SplitMessage(long, 50)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	reassembled := ""
	for _, c := range chunks {
		reassembled += c
	}
	if reassembled != long {
		t.Fatalf("reassembled text does not match original")
	}
}

func TestSplitMessage_SplitsAtNewline(t *testing.T) {
	text := "aaa\nbbb\nccc\n"
	chunks := SplitMessage(text, 8)
	for _, c := range chunks {
		if len(c) > 8 {
			t.Fatalf("chunk exceeds max: %q", c)
		}
	}
	reassembled := ""
	for _, c := range chunks {
		reassembled += c
	}
	if reassembled != text {
		t.Fatalf("reassembled mismatch")
	}
}

func TestSplitMessage_CJK(t *testing.T) {
	// Each CJK character is 3 bytes in UTF-8; test that we never cut mid-rune
	text := "你好世界这是一个测试消息用来验证UTF8分割"
	chunks := SplitMessage(text, 10)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	reassembled := ""
	for _, c := range chunks {
		if !utf8.ValidString(c) {
			t.Fatalf("chunk contains invalid UTF-8: %q", c)
		}
		reassembled += c
	}
	if reassembled != text {
		t.Fatalf("reassembled CJK text does not match original")
	}
}
