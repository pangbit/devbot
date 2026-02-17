package bot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type ExecResult struct {
	Output             string
	SessionID          string
	IsPermissionDenial bool
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

	rawOut := stdout.String()
	log.Printf("claude: raw output len=%d session=%s", len(rawOut), sessionID)
	if len(rawOut) < 3000 {
		log.Printf("claude: raw json: %s", rawOut)
	} else {
		log.Printf("claude: raw json (truncated): %s", rawOut[:3000])
	}

	var resp struct {
		Result           string `json:"result"`
		SessionID        string `json:"session_id"`
		IsError          bool   `json:"is_error"`
		Subtype          string `json:"subtype"`
		PermissionDenials []permissionDenial `json:"permission_denials"`
	}
	if err := json.Unmarshal([]byte(rawOut), &resp); err != nil {
		return ExecResult{}, fmt.Errorf("failed to parse claude response: %w\nraw: %s", err, rawOut)
	}
	log.Printf("claude: parsed result_len=%d session_id=%s is_error=%v subtype=%s denials=%d", len(resp.Result), resp.SessionID, resp.IsError, resp.Subtype, len(resp.PermissionDenials))
	if resp.IsError {
		errMsg := resp.Result
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return ExecResult{SessionID: resp.SessionID}, fmt.Errorf("claude error: %s", errMsg)
	}

	output := resp.Result
	// When result is empty but there are permission denials, extract the denied content
	if output == "" && len(resp.PermissionDenials) > 0 {
		output = formatPermissionDenials(resp.PermissionDenials)
		return ExecResult{Output: output, SessionID: resp.SessionID, IsPermissionDenial: true}, nil
	}
	return ExecResult{Output: output, SessionID: resp.SessionID}, nil
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

type permissionDenial struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

func formatPermissionDenials(denials []permissionDenial) string {
	var sb strings.Builder
	sb.WriteString("Claude 想向你确认：\n\n")
	for _, d := range denials {
		if d.ToolName == "AskUserQuestion" {
			sb.WriteString(formatAskUserQuestion(d.ToolInput))
		} else {
			sb.WriteString(fmt.Sprintf("(blocked: %s)\n", d.ToolName))
		}
	}
	return sb.String()
}

func formatAskUserQuestion(input json.RawMessage) string {
	var ask struct {
		Questions []struct {
			Question string `json:"question"`
			Options  []struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"options"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(input, &ask); err != nil {
		return string(input)
	}
	var sb strings.Builder
	for _, q := range ask.Questions {
		sb.WriteString(q.Question)
		sb.WriteString("\n\n")
		for i, opt := range q.Options {
			sb.WriteString(fmt.Sprintf("%d. %s\n   %s\n", i+1, opt.Label, opt.Description))
		}
		sb.WriteString("\n请回复选项编号继续。")
	}
	return sb.String()
}

type streamEvent struct {
	Type              string             `json:"type"`
	Result            string             `json:"result"`
	SessionID         string             `json:"session_id"`
	IsError           bool               `json:"is_error"`
	Subtype           string             `json:"subtype"`
	PermissionDenials []permissionDenial `json:"permission_denials"`
	Message           json.RawMessage    `json:"message"`
	Errors            []string           `json:"errors"`
}

func extractAssistantText(msg json.RawMessage) string {
	if msg == nil {
		return ""
	}
	var m struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(msg, &m); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, c := range m.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

// ExecStream runs Claude CLI with streaming output (stream-json).
// It calls onProgress with the text from each assistant message during execution.
// Returns the final ExecResult when done.
func (c *ClaudeExecutor) ExecStream(ctx context.Context, prompt, workDir, sessionID, permissionMode, model string, onProgress func(text string)) (ExecResult, error) {
	args := []string{"-p", prompt, "--output-format", "stream-json", "--verbose"}
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

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ExecResult{}, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return ExecResult{}, fmt.Errorf("failed to start claude: %w", err)
	}

	c.mu.Lock()
	c.running = cmd
	c.mu.Unlock()

	start := time.Now()

	var result ExecResult
	var gotResult bool
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var ev streamEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			log.Printf("claude stream: failed to parse line: %v", err)
			continue
		}

		switch ev.Type {
		case "assistant":
			text := extractAssistantText(ev.Message)
			if text != "" && onProgress != nil {
				onProgress(text)
			}
		case "system":
			log.Printf("claude stream: system event: %s", string(line))
		case "result":
			result.Output = ev.Result
			result.SessionID = ev.SessionID
			result.IsPermissionDenial = len(ev.PermissionDenials) > 0
			if result.Output == "" && result.IsPermissionDenial {
				result.Output = formatPermissionDenials(ev.PermissionDenials)
			}
			if ev.IsError {
				duration := time.Since(start)
				cmd.Wait()
				c.mu.Lock()
				c.running = nil
				c.execCount++
				c.lastExecDuration = duration
				c.mu.Unlock()
				log.Printf("claude stream: error result raw: %s", string(line))
				errMsg := ev.Result
				if errMsg == "" && len(ev.Errors) > 0 {
					errMsg = strings.Join(ev.Errors, "; ")
				}
				if errMsg == "" {
					errMsg = "unknown error"
				}
				if stderrStr := stderr.String(); stderrStr != "" {
					log.Printf("claude stream: stderr: %s", stderrStr)
					return ExecResult{SessionID: ev.SessionID}, fmt.Errorf("claude error: %s\nstderr: %s", errMsg, stderrStr)
				}
				return ExecResult{SessionID: ev.SessionID}, fmt.Errorf("claude error: %s", errMsg)
			}
			gotResult = true
		}
	}

	duration := time.Since(start)
	waitErr := cmd.Wait()

	c.mu.Lock()
	c.running = nil
	c.execCount++
	c.lastExecDuration = duration
	c.mu.Unlock()

	if !gotResult {
		if ctx.Err() == context.DeadlineExceeded {
			return ExecResult{}, fmt.Errorf("execution timed out after %v", c.timeout)
		}
		if waitErr != nil {
			return ExecResult{}, fmt.Errorf("claude error: %w\nstderr: %s", waitErr, stderr.String())
		}
		return ExecResult{}, fmt.Errorf("no result event in stream output")
	}

	return result, nil
}
