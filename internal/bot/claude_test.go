package bot

import (
	"context"
	"encoding/json"
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

func TestClaudeExecInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\necho 'not valid json'\n"), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, err := exec.Exec(context.Background(), "test", dir, "", "safe", "sonnet")
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestClaudeExecEmptySessionID(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"Hello"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	result, err := exec.Exec(context.Background(), "test", dir, "", "safe", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if result.Output != "Hello" {
		t.Fatalf("unexpected output: %q", result.Output)
	}
	if result.SessionID != "" {
		t.Fatalf("expected empty session ID, got: %q", result.SessionID)
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

func TestClaudeExecStream_CollectsResult(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello world"}]},"session_id":"s1"}'
echo '{"type":"result","subtype":"success","result":"Hello world","session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	result, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err != nil {
		t.Fatalf("ExecStream error: %v", err)
	}
	if result.Output != "Hello world" {
		t.Fatalf("unexpected output: %q", result.Output)
	}
	if result.SessionID != "s1" {
		t.Fatalf("unexpected session ID: %q", result.SessionID)
	}
}

func TestClaudeExecStream_Timeout(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\nsleep 10\n"), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 100*time.Millisecond)
	_, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
}

func TestClaudeExecStream_ErrorResult(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"something failed","is_error":true,"session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "something failed") {
		t.Fatalf("expected error message, got: %v", err)
	}
}

func TestClaudeExecStream_ProgressCallback(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"step 1"}]},"session_id":"s1"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"step 2"}]},"session_id":"s1"}'
echo '{"type":"result","result":"done","session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	var progTexts []string
	result, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", func(text string) {
		progTexts = append(progTexts, text)
	})
	if err != nil {
		t.Fatalf("ExecStream error: %v", err)
	}
	if result.Output != "done" {
		t.Fatalf("unexpected output: %q", result.Output)
	}
	if len(progTexts) < 2 {
		t.Fatalf("expected at least 2 progress callbacks, got %d", len(progTexts))
	}
}

func TestClaudeExecStream_PermissionDenial(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"","session_id":"s1","permission_denials":[{"tool_name":"Bash","tool_input":{}}]}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	result, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err != nil {
		t.Fatalf("ExecStream error: %v", err)
	}
	if !result.IsPermissionDenial {
		t.Fatalf("expected IsPermissionDenial=true")
	}
}

func TestExtractAssistantText_Nil(t *testing.T) {
	result := extractAssistantText(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil message, got %q", result)
	}
}

func TestExtractAssistantText_InvalidJSON(t *testing.T) {
	result := extractAssistantText([]byte("{not valid json"))
	if result != "" {
		t.Fatalf("expected empty string for invalid JSON, got %q", result)
	}
}

func TestExtractAssistantText_NonTextContent(t *testing.T) {
	// Content with only non-text types should return empty string
	msg := []byte(`{"content":[{"type":"tool_use","id":"t1"},{"type":"text","text":"hello"}]}`)
	result := extractAssistantText(msg)
	if result != "hello" {
		t.Fatalf("expected 'hello', got %q", result)
	}
}

func TestClaudeExec_ErrorEmptyResult(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	// is_error=true but result is empty → should use "unknown error"
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"","is_error":true,"session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, err := exec.Exec(context.Background(), "test", dir, "", "safe", "sonnet")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unknown error") {
		t.Fatalf("expected 'unknown error', got: %v", err)
	}
}

func TestClaudeExecStream_ErrorWithStderr(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	// Writes to stderr and outputs an error result
	os.WriteFile(script, []byte(`#!/bin/sh
echo "something went wrong" >&2
echo '{"type":"result","result":"exec failed","is_error":true,"session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("expected 'exec failed' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Fatalf("expected stderr in error, got: %v", err)
	}
}

func TestClaudeExecStream_NoResult(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	// Script outputs something but never a result event → "no result event" error
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"system","subtype":"init"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err == nil {
		t.Fatalf("expected error for no result event")
	}
	if !strings.Contains(err.Error(), "no result event") {
		t.Fatalf("expected 'no result event' error, got: %v", err)
	}
}

func TestClaudeExecStream_CountAndDuration(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"ok","session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	before := exec.ExecCount()
	_, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err != nil {
		t.Fatalf("ExecStream error: %v", err)
	}
	if exec.ExecCount() != before+1 {
		t.Fatalf("expected exec count to increment")
	}
	if exec.LastExecDuration() <= 0 {
		t.Fatalf("expected positive duration")
	}
}

func TestClaudeExec_YoloMode(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\necho \"$@\" > %s\necho '{\"type\":\"result\",\"result\":\"ok\",\"session_id\":\"s1\"}'\n", argsFile)), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, err := exec.Exec(context.Background(), "test", dir, "", "yolo", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	argsData, _ := os.ReadFile(argsFile)
	if !strings.Contains(string(argsData), "--dangerously-skip-permissions") {
		t.Fatalf("expected --dangerously-skip-permissions in args, got: %q", string(argsData))
	}
}

func TestClaudeExec_LargeOutput(t *testing.T) {
	// Output >= 3000 chars triggers the truncated log path.
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	large := strings.Repeat("x", 3001)
	os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\nprintf '{\"type\":\"result\",\"result\":\"%s\",\"session_id\":\"s1\"}\\n' \n", large)), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	result, err := exec.Exec(context.Background(), "test", dir, "", "safe", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if len(result.Output) < 3000 {
		t.Fatalf("expected large output, got len=%d", len(result.Output))
	}
}

func TestClaudeExec_PermissionDenial(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"","session_id":"s1","permission_denials":[{"tool_name":"Bash","tool_input":{"command":"ls"}}]}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	result, err := exec.Exec(context.Background(), "test", dir, "", "safe", "sonnet")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if !result.IsPermissionDenial {
		t.Fatalf("expected IsPermissionDenial=true")
	}
}

func TestClaudeExecStream_YoloMode(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\necho \"$@\" > %s\necho '{\"type\":\"result\",\"result\":\"ok\",\"session_id\":\"s1\"}'\n", argsFile)), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, err := exec.ExecStream(context.Background(), "test", dir, "", "yolo", "sonnet", nil)
	if err != nil {
		t.Fatalf("ExecStream error: %v", err)
	}
	argsData, _ := os.ReadFile(argsFile)
	if !strings.Contains(string(argsData), "--dangerously-skip-permissions") {
		t.Fatalf("expected --dangerously-skip-permissions in args, got: %q", string(argsData))
	}
}

func TestClaudeExecStream_EmptyLines(t *testing.T) {
	// Empty lines should be skipped without panic.
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo ""
echo ""
echo '{"type":"result","result":"ok","session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	result, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err != nil {
		t.Fatalf("ExecStream error: %v", err)
	}
	if result.Output != "ok" {
		t.Fatalf("unexpected output: %q", result.Output)
	}
}

func TestClaudeExecStream_NonJSONLine(t *testing.T) {
	// Non-JSON lines should be skipped (logged), not cause an error.
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo "this is not json"
echo '{"type":"result","result":"ok","session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	result, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err != nil {
		t.Fatalf("ExecStream error: %v", err)
	}
	if result.Output != "ok" {
		t.Fatalf("unexpected output: %q", result.Output)
	}
}

func TestClaudeExecStream_ErrorWithErrors(t *testing.T) {
	// Error result with errors array (ev.Result="" but ev.Errors has items).
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"","is_error":true,"session_id":"s1","errors":["err1","err2"]}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "err1") || !strings.Contains(err.Error(), "err2") {
		t.Fatalf("expected errors array in error message, got: %v", err)
	}
}

func TestClaudeExecStream_ErrorEmpty(t *testing.T) {
	// Error result with empty result and empty errors → "unknown error".
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"","is_error":true,"session_id":"s1"}'
`), 0755)

	exec := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	_, err := exec.ExecStream(context.Background(), "test", dir, "", "safe", "sonnet", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unknown error") {
		t.Fatalf("expected 'unknown error', got: %v", err)
	}
}

func TestFormatAskUserQuestion_InvalidJSON(t *testing.T) {
	// Invalid JSON should return the raw input as a string.
	input := json.RawMessage(`not valid json`)
	result := formatAskUserQuestion(input)
	if result != "not valid json" {
		t.Fatalf("expected raw input returned for invalid JSON, got %q", result)
	}
}

func TestClaudeExec_StartError(t *testing.T) {
	// Nonexistent binary causes cmd.Start() to fail.
	exec := NewClaudeExecutor("/nonexistent_binary_xyz_for_test", "sonnet", 30*time.Second)
	_, err := exec.Exec(context.Background(), "test", t.TempDir(), "", "safe", "sonnet")
	if err == nil {
		t.Fatalf("expected error for nonexistent binary")
	}
	if !strings.Contains(err.Error(), "failed to start claude") {
		t.Fatalf("expected 'failed to start claude' error, got: %v", err)
	}
}
