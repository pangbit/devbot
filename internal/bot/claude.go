package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

type ExecResult struct {
	Output    string
	SessionID string
}

type ClaudeExecutor struct {
	claudePath       string
	model            string
	timeout          time.Duration
	mu               sync.Mutex
	running          *exec.Cmd
	lastExecDuration time.Duration
	execCount        int
}

func NewClaudeExecutor(claudePath, model string, timeout time.Duration) *ClaudeExecutor {
	return &ClaudeExecutor{
		claudePath: claudePath,
		model:      model,
		timeout:    timeout,
	}
}

func (c *ClaudeExecutor) Exec(ctx context.Context, prompt, workDir, sessionID, permissionMode, model string) (ExecResult, error) {
	args := []string{"-p", prompt, "--output-format", "json"}
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if permissionMode == "yolo" {
		args = append(args, "--dangerously-skip-permissions")
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.claudePath, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return ExecResult{}, fmt.Errorf("failed to start claude: %w", err)
	}

	c.mu.Lock()
	c.running = cmd
	c.mu.Unlock()

	start := time.Now()
	err := cmd.Wait()
	duration := time.Since(start)

	c.mu.Lock()
	c.running = nil
	c.execCount++
	c.lastExecDuration = duration
	c.mu.Unlock()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ExecResult{}, fmt.Errorf("execution timed out after %v", c.timeout)
		}
		return ExecResult{}, fmt.Errorf("claude error: %w\nstderr: %s", err, stderr.String())
	}

	log.Printf("claude: raw output len=%d session=%s", stdout.Len(), sessionID)

	var resp struct {
		Result    string `json:"result"`
		SessionID string `json:"session_id"`
		IsError   bool   `json:"is_error"`
		Subtype   string `json:"subtype"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return ExecResult{}, fmt.Errorf("failed to parse claude response: %w\nraw: %s", err, stdout.String())
	}
	log.Printf("claude: parsed result_len=%d session_id=%s is_error=%v subtype=%s", len(resp.Result), resp.SessionID, resp.IsError, resp.Subtype)
	if resp.IsError {
		errMsg := resp.Result
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return ExecResult{SessionID: resp.SessionID}, fmt.Errorf("claude error: %s", errMsg)
	}
	return ExecResult{Output: resp.Result, SessionID: resp.SessionID}, nil
}

func (c *ClaudeExecutor) Kill() error {
	c.mu.Lock()
	cmd := c.running
	c.mu.Unlock()

	if cmd == nil {
		return fmt.Errorf("no running process")
	}
	return cmd.Process.Kill()
}

func (c *ClaudeExecutor) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running != nil
}

func (c *ClaudeExecutor) SetModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.model = model
}

func (c *ClaudeExecutor) Model() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.model
}

// WaitIdle blocks until no execution is running or the timeout expires.
// Returns true if idle, false if timed out.
func (c *ClaudeExecutor) WaitIdle(timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		if !c.IsRunning() {
			return true
		}
		select {
		case <-deadline:
			return false
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (c *ClaudeExecutor) ExecCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.execCount
}

func (c *ClaudeExecutor) LastExecDuration() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastExecDuration
}
