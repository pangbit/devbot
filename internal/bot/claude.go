package bot

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

type ClaudeExecutor struct {
	claudePath string
	model      string
	timeout    time.Duration
	mu         sync.Mutex
	running    *exec.Cmd
}

func NewClaudeExecutor(claudePath, model string, timeout time.Duration) *ClaudeExecutor {
	return &ClaudeExecutor{
		claudePath: claudePath,
		model:      model,
		timeout:    timeout,
	}
}

func (c *ClaudeExecutor) Exec(ctx context.Context, prompt, workDir, sessionID, permissionMode, model string) (string, error) {
	args := []string{"-p", prompt, "--output-format", "text"}
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

	c.mu.Lock()
	c.running = cmd
	c.mu.Unlock()

	err := cmd.Run()

	c.mu.Lock()
	c.running = nil
	c.mu.Unlock()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("execution timed out after %v", c.timeout)
		}
		return "", fmt.Errorf("claude error: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func (c *ClaudeExecutor) Kill() error {
	c.mu.Lock()
	cmd := c.running
	c.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
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
