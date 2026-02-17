package bot

import "testing"

func TestCardMsg(t *testing.T) {
	card := CardMsg{Title: "Test", Content: "**bold** `code`"}
	if card.Title != "Test" {
		t.Fatalf("Title mismatch: %q", card.Title)
	}
	if card.Content != "**bold** `code`" {
		t.Fatalf("Content mismatch: %q", card.Content)
	}
}
