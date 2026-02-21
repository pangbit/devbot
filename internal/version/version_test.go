package version

import (
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	s := String()
	if !strings.Contains(s, "devbot version") {
		t.Fatalf("expected 'devbot version' in output, got: %q", s)
	}
	if !strings.Contains(s, "Commit:") {
		t.Fatalf("expected 'Commit:' in output, got: %q", s)
	}
	if !strings.Contains(s, "Built:") {
		t.Fatalf("expected 'Built:' in output, got: %q", s)
	}
}
