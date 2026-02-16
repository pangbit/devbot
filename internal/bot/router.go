package bot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Router struct {
	executor     *ClaudeExecutor
	store        *Store
	sender       Sender
	allowedUsers map[string]bool
	startTime    time.Time
	queue        *MessageQueue
}

func NewRouter(executor *ClaudeExecutor, store *Store, sender Sender, allowedUsers map[string]bool, workRoot string) *Router {
	store.State().WorkRoot = workRoot
	return &Router{
		executor:     executor,
		store:        store,
		sender:       sender,
		allowedUsers: allowedUsers,
		startTime:    time.Now(),
	}
}

func (r *Router) SetQueue(q *MessageQueue) {
	r.queue = q
}

func (r *Router) Route(ctx context.Context, chatID, userID, text string) {
	if !r.allowedUsers[userID] {
		return
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	if strings.HasPrefix(text, "/") {
		r.handleCommand(ctx, chatID, text)
		return
	}

	r.handlePrompt(ctx, chatID, text)
}

func (r *Router) handleCommand(ctx context.Context, chatID, text string) {
	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/help":
		r.cmdHelp(ctx, chatID)
	case "/ping":
		r.cmdPing(ctx, chatID)
	case "/status":
		r.cmdStatus(ctx, chatID)
	case "/pwd":
		r.cmdPwd(ctx, chatID)
	case "/ls":
		r.cmdLs(ctx, chatID)
	case "/root":
		r.cmdRoot(ctx, chatID, args)
	case "/cd":
		r.cmdCd(ctx, chatID, args)
	case "/new":
		r.cmdNewSession(ctx, chatID)
	case "/sessions":
		r.cmdSessions(ctx, chatID)
	case "/switch":
		r.cmdSwitch(ctx, chatID, args)
	case "/kill":
		r.cmdKill(ctx, chatID)
	case "/model":
		r.cmdModel(ctx, chatID, args)
	case "/yolo":
		r.cmdYolo(ctx, chatID)
	case "/safe":
		r.cmdSafe(ctx, chatID)
	case "/last":
		r.cmdLast(ctx, chatID)
	case "/summary":
		r.cmdSummary(ctx, chatID)
	case "/git":
		r.cmdGit(ctx, chatID, args)
	case "/diff":
		r.cmdGit(ctx, chatID, "diff")
	case "/commit":
		r.cmdCommit(ctx, chatID, args)
	case "/push":
		r.cmdGit(ctx, chatID, "push")
	case "/undo":
		r.cmdGit(ctx, chatID, "checkout .")
	case "/stash":
		if args == "" {
			r.cmdGit(ctx, chatID, "stash")
		} else {
			r.cmdGit(ctx, chatID, "stash "+args)
		}
	case "/sh":
		r.cmdSh(ctx, chatID, args)
	case "/file":
		r.cmdFile(ctx, chatID, args)
	case "/doc":
		r.cmdDoc(ctx, chatID, args)
	default:
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Unknown command: %s\nUse /help to see available commands.", cmd))
	}
}

func (r *Router) getSession(chatID string) *Session {
	state := r.store.State()
	s := state.Chats[chatID]
	if s == nil {
		s = &Session{
			WorkDir: state.WorkRoot,
			Model:   r.executor.Model(),
		}
		state.Chats[chatID] = s
	}
	return s
}

func (r *Router) cmdHelp(ctx context.Context, chatID string) {
	help := `Available commands:

Basic:
  /help        -- Show this help
  /ping        -- Check if bot is alive
  /status      -- Show current status and Claude usage

Directory:
  /root <path> -- Set root work directory
  /cd <dir>    -- Change directory (relative to root)
  /pwd         -- Show current directory
  /ls          -- List projects in root directory

Git:
  /git <args>  -- Run git command
  /diff        -- Show current changes
  /commit <msg>-- Quick commit
  /push        -- Quick push
  /undo        -- Discard uncommitted changes
  /stash [pop] -- Stash/restore changes

Session:
  /new         -- Start new Claude session
  /sessions    -- List session history
  /switch <id> -- Switch to session

Control:
  /kill        -- Terminate current execution
  /model <name>-- Switch Claude model
  /yolo        -- Enable unrestricted mode
  /safe        -- Restore safe mode
  /last        -- Show last Claude output
  /summary     -- Summarize last output

System:
  /sh <cmd>    -- Run shell command (via Claude)
  /file <path> -- Send project file to chat

Docs:
  /doc push <path>  -- Push Markdown to Feishu doc
  /doc pull         -- Pull shared doc to project
  /doc bind <path>  -- Bind file to Feishu doc
  /doc unbind <path>-- Unbind
  /doc list         -- List bindings

Any other message is sent directly to Claude as a prompt.`
	r.sender.SendText(ctx, chatID, help)
}

func (r *Router) cmdPing(ctx context.Context, chatID string) {
	uptime := time.Since(r.startTime).Truncate(time.Second)
	r.sender.SendText(ctx, chatID, fmt.Sprintf("pong (uptime: %s)", uptime))
}

func (r *Router) cmdStatus(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	uptime := time.Since(r.startTime).Truncate(time.Second)
	mode := session.PermissionMode
	if mode == "" {
		mode = "safe"
	}

	status := fmt.Sprintf(`Status:
  WorkDir:  %s
  Session:  %s
  Model:    %s
  Mode:     %s
  Running:  %v
  uptime:   %s`,
		session.WorkDir,
		session.ClaudeSessionID,
		session.Model,
		mode,
		r.executor.IsRunning(),
		uptime,
	)
	r.sender.SendText(ctx, chatID, status)
}

func (r *Router) cmdPwd(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	r.sender.SendText(ctx, chatID, session.WorkDir)
}

func (r *Router) cmdLs(ctx context.Context, chatID string) {
	root := r.store.State().WorkRoot
	entries, err := os.ReadDir(root)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		r.sender.SendText(ctx, chatID, "No projects found in "+root)
		return
	}
	r.sender.SendText(ctx, chatID, strings.Join(dirs, "\n"))
}

func (r *Router) cmdRoot(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "Current root: "+r.store.State().WorkRoot)
		return
	}
	if _, err := os.Stat(args); err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Directory not found: %s", args))
		return
	}
	r.store.State().WorkRoot = args
	r.store.Save()
	r.sender.SendText(ctx, chatID, "Root set to: "+args)
}

func (r *Router) cmdCd(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "Usage: /cd <directory>")
		return
	}
	session := r.getSession(chatID)
	root := r.store.State().WorkRoot

	var target string
	if filepath.IsAbs(args) {
		target = args
	} else {
		target = filepath.Join(root, args)
	}
	target = filepath.Clean(target)

	if _, err := os.Stat(target); err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Directory not found: %s", target))
		return
	}
	session.WorkDir = target
	r.store.Save()
	r.sender.SendText(ctx, chatID, "Changed to: "+target)
}

func (r *Router) cmdNewSession(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	if session.ClaudeSessionID != "" {
		session.History = append(session.History, session.ClaudeSessionID)
	}
	session.ClaudeSessionID = ""
	session.LastOutput = ""
	r.store.Save()
	r.sender.SendText(ctx, chatID, "New session started.")
}

func (r *Router) cmdSessions(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	if len(session.History) == 0 && session.ClaudeSessionID == "" {
		r.sender.SendText(ctx, chatID, "No sessions.")
		return
	}
	var lines []string
	for i, id := range session.History {
		lines = append(lines, fmt.Sprintf("  %d: %s", i, id))
	}
	if session.ClaudeSessionID != "" {
		lines = append(lines, fmt.Sprintf("  * %s (current)", session.ClaudeSessionID))
	}
	r.sender.SendText(ctx, chatID, "Sessions:\n"+strings.Join(lines, "\n"))
}

func (r *Router) cmdSwitch(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "Usage: /switch <session-id>")
		return
	}
	session := r.getSession(chatID)
	if session.ClaudeSessionID != "" {
		session.History = append(session.History, session.ClaudeSessionID)
	}
	session.ClaudeSessionID = args
	session.LastOutput = ""
	r.store.Save()
	r.sender.SendText(ctx, chatID, "Switched to session: "+args)
}

func (r *Router) cmdKill(ctx context.Context, chatID string) {
	if err := r.executor.Kill(); err != nil {
		r.sender.SendText(ctx, chatID, "No running task to kill.")
		return
	}
	r.sender.SendText(ctx, chatID, "Task killed.")
}

func (r *Router) cmdModel(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "Current model: "+r.executor.Model())
		return
	}
	session := r.getSession(chatID)
	session.Model = args
	r.executor.SetModel(args)
	r.store.Save()
	r.sender.SendText(ctx, chatID, "Model set to: "+args)
}

func (r *Router) cmdYolo(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	session.PermissionMode = "yolo"
	r.store.Save()
	r.sender.SendText(ctx, chatID, "YOLO mode enabled. Claude will execute without restrictions.")
}

func (r *Router) cmdSafe(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	session.PermissionMode = "safe"
	r.store.Save()
	r.sender.SendText(ctx, chatID, "Safe mode restored.")
}

func (r *Router) cmdLast(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	if session.LastOutput == "" {
		r.sender.SendText(ctx, chatID, "No previous output.")
		return
	}
	r.sender.SendTextChunked(ctx, chatID, session.LastOutput)
}

func (r *Router) cmdSummary(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	if session.LastOutput == "" {
		r.sender.SendText(ctx, chatID, "No previous output to summarize.")
		return
	}
	prompt := "Please summarize the following output concisely:\n\n" + session.LastOutput
	r.execClaudeQueued(ctx, chatID, session, prompt)
}

func (r *Router) cmdCommit(ctx context.Context, chatID, msg string) {
	if msg == "" {
		r.sender.SendText(ctx, chatID, "Usage: /commit <message>")
		return
	}
	session := r.getSession(chatID)
	prompt := fmt.Sprintf("Stage all changes with `git add -A`, then commit with the message: %s\nOnly show the command output, no explanation.", msg)
	r.execClaudeQueued(ctx, chatID, session, prompt)
}

func (r *Router) cmdGit(ctx context.Context, chatID, args string) {
	session := r.getSession(chatID)
	prompt := fmt.Sprintf("Run `git %s` in the current directory and return the output. Only show the command output, no explanation.", args)
	r.execClaudeQueued(ctx, chatID, session, prompt)
}

func (r *Router) cmdSh(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "Usage: /sh <command>")
		return
	}
	session := r.getSession(chatID)
	prompt := fmt.Sprintf("Run `%s` in the current directory and return the output. Only show the command output, no explanation.", args)
	r.execClaudeQueued(ctx, chatID, session, prompt)
}

func (r *Router) cmdFile(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "Usage: /file <path>")
		return
	}
	session := r.getSession(chatID)
	target := findFile(session.WorkDir, args)
	if target == "" {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("File not found: %s", args))
		return
	}
	data, err := os.ReadFile(target)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Error reading file: %v", err))
		return
	}
	r.sender.SendTextChunked(ctx, chatID, fmt.Sprintf("%s:\n\n%s", filepath.Base(target), string(data)))
}

func findFile(workDir, query string) string {
	if filepath.IsAbs(query) {
		if _, err := os.Stat(query); err == nil {
			return query
		}
		return ""
	}
	exact := filepath.Join(workDir, query)
	if _, err := os.Stat(exact); err == nil {
		return exact
	}
	query = strings.ToLower(query)
	var match string
	filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.Contains(strings.ToLower(info.Name()), query) {
			match = path
			return filepath.SkipAll
		}
		return nil
	})
	return match
}

func (r *Router) cmdDoc(ctx context.Context, chatID, args string) {
	r.sender.SendText(ctx, chatID, "Doc command not yet implemented.")
}

func (r *Router) handlePrompt(ctx context.Context, chatID, text string) {
	session := r.getSession(chatID)
	r.execClaudeQueued(ctx, chatID, session, text)
}

func (r *Router) execClaudeQueued(ctx context.Context, chatID string, session *Session, prompt string) {
	if r.queue != nil {
		pending := r.queue.PendingCount(chatID)
		if pending > 0 {
			r.sender.SendText(ctx, chatID, fmt.Sprintf("Queued (position %d)...", pending+1))
		}
		done := make(chan struct{})
		r.queue.Enqueue(chatID, func() {
			defer close(done)
			r.execClaude(ctx, chatID, session, prompt)
		})
		<-done
	} else {
		r.execClaude(ctx, chatID, session, prompt)
	}
}

func (r *Router) execClaude(ctx context.Context, chatID string, session *Session, prompt string) {
	r.sender.SendText(ctx, chatID, "Executing...")

	permMode := session.PermissionMode
	if permMode == "" {
		permMode = "safe"
	}
	output, err := r.executor.Exec(ctx, prompt, session.WorkDir, session.ClaudeSessionID, permMode)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	session.LastOutput = output
	r.store.Save()

	r.sender.SendTextChunked(ctx, chatID, output)
}
