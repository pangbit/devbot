package bot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaudeExecRunsCommand(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"Hello from Claude\"\n"), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	output, err := exec.Exec(context.Background(), "test prompt", dir, "", "safe", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if output != "Hello from Claude\n" {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestClaudeExecTimeout(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\nsleep 10\n"), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 100*time.Millisecond)
	_, err := exec.Exec(context.Background(), "test", dir, "", "safe", "sonnet")
	if err == nil {
		t.Fatalf("expected timeout error")
	}
}

func TestClaudeExecWithSessionID(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\"\n"), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	output, err := exec.Exec(context.Background(), "hello", dir, "ses123", "safe", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if !strings.Contains(output, "--resume") || !strings.Contains(output, "ses123") {
		t.Fatalf("expected --resume ses123 in args, got: %q", output)
	}
}

func TestClaudeExecKillAndIsRunning(t *testing.T) {
	// Kill with no running process should return error
	exec := NewClaudeExecutor("claude", "sonnet", 30*time.Second)
	if err := exec.Kill(); err == nil {
		t.Fatalf("expected error when killing with no process")
	}
	if exec.IsRunning() {
		t.Fatalf("expected not running")
	}
}

func TestClaudeExecModelGetSet(t *testing.T) {
	exec := NewClaudeExecutor("claude", "sonnet", 30*time.Second)
	if exec.Model() != "sonnet" {
		t.Fatalf("expected sonnet, got %q", exec.Model())
	}
	exec.SetModel("opus")
	if exec.Model() != "opus" {
		t.Fatalf("expected opus, got %q", exec.Model())
	}
}
