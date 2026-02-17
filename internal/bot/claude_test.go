package bot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaudeExecRunsCommand(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"Hello from Claude","session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	result, err := exec.Exec(context.Background(), "test prompt", dir, "", "safe", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if result.Output != "Hello from Claude" {
		t.Fatalf("unexpected output: %q", result.Output)
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
	argsFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\necho \"$@\" > %s\necho '{\"type\":\"result\",\"result\":\"ok\",\"session_id\":\"s1\"}'\n", argsFile)), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, err := exec.Exec(context.Background(), "hello", dir, "ses123", "safe", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	argsData, _ := os.ReadFile(argsFile)
	args := string(argsData)
	if !strings.Contains(args, "--resume") || !strings.Contains(args, "ses123") {
		t.Fatalf("expected --resume ses123 in args, got: %q", args)
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

func TestClaudeExecCountAndDuration(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"ok","session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)

	// Before any execution
	if exec.ExecCount() != 0 {
		t.Fatalf("expected 0 exec count, got %d", exec.ExecCount())
	}
	if exec.LastExecDuration() != 0 {
		t.Fatalf("expected 0 duration, got %v", exec.LastExecDuration())
	}

	// After first execution
	_, err := exec.Exec(context.Background(), "test", dir, "", "safe", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if exec.ExecCount() != 1 {
		t.Fatalf("expected 1 exec count, got %d", exec.ExecCount())
	}
	if exec.LastExecDuration() <= 0 {
		t.Fatalf("expected positive duration, got %v", exec.LastExecDuration())
	}

	// After second execution
	_, err = exec.Exec(context.Background(), "test2", dir, "", "safe", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if exec.ExecCount() != 2 {
		t.Fatalf("expected 2 exec count, got %d", exec.ExecCount())
	}
}

func TestClaudeExecWaitIdleImmediate(t *testing.T) {
	exec := NewClaudeExecutor("claude", "sonnet", 30*time.Second)
	// Not running, should return immediately
	if !exec.WaitIdle(1 * time.Second) {
		t.Fatalf("expected WaitIdle to return true when not running")
	}
}

func TestClaudeExecWaitIdleTimeout(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)

	// Start a long-running command in background
	go exec.Exec(context.Background(), "test", dir, "", "safe", "sonnet")
	time.Sleep(50 * time.Millisecond) // Let it start

	// Should timeout
	if exec.WaitIdle(200 * time.Millisecond) {
		t.Fatalf("expected WaitIdle to return false (timeout)")
	}

	// Cleanup
	exec.Kill()
}

func TestClaudeExecReturnsExecResult(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	// Fake CLI that returns JSON with session_id
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"Hello from Claude","session_id":"sess-abc-123"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	result, err := exec.Exec(context.Background(), "test prompt", dir, "", "safe", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if result.Output != "Hello from Claude" {
		t.Fatalf("unexpected output: %q", result.Output)
	}
	if result.SessionID != "sess-abc-123" {
		t.Fatalf("unexpected session ID: %q", result.SessionID)
	}
}

func TestClaudeExecCountIncrementsOnError(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, _ = exec.Exec(context.Background(), "test", dir, "", "safe", "sonnet")

	// Count should still increment even on error
	if exec.ExecCount() != 1 {
		t.Fatalf("expected 1 exec count after error, got %d", exec.ExecCount())
	}
	if exec.LastExecDuration() <= 0 {
		t.Fatalf("expected positive duration after error, got %v", exec.LastExecDuration())
	}
}
