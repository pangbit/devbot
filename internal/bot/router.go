package bot

import (
	"context"
	"fmt"
	"log"
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
	docSyncer    DocPusher
	ctx          context.Context
}

func NewRouter(ctx context.Context, executor *ClaudeExecutor, store *Store, sender Sender, allowedUsers map[string]bool, workRoot string, docSyncer DocPusher) *Router {
	if store.WorkRoot() == "" {
		store.SetWorkRoot(workRoot)
	}
	return &Router{
		executor:     executor,
		store:        store,
		sender:       sender,
		allowedUsers: allowedUsers,
		startTime:    time.Now(),
		docSyncer:    docSyncer,
		ctx:          ctx,
	}
}

func (r *Router) SetQueue(q *MessageQueue) {
	r.queue = q
}

func (r *Router) save() {
	if err := r.store.Save(); err != nil {
		log.Printf("router: failed to save state: %v", err)
	}
}

func (r *Router) Route(ctx context.Context, chatID, userID, text string) {
	if !r.allowedUsers[userID] {
		log.Printf("router: unauthorized user=%s, ignoring", userID)
		return
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	if strings.HasPrefix(text, "/") {
		log.Printf("router: command %s from chat=%s", strings.SplitN(text, " ", 2)[0], chatID)
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

func (r *Router) getSession(chatID string) Session {
	return r.store.GetSession(chatID, r.store.WorkRoot(), r.executor.Model())
}

func (r *Router) cmdHelp(ctx context.Context, chatID string) {
	md := "**Basic:**\n" +
		"`/help`  Show this help\n" +
		"`/ping`  Check if bot is alive\n" +
		"`/status`  Show current status and Claude usage\n\n" +
		"**Directory:**\n" +
		"`/root <path>`  Set root work directory\n" +
		"`/cd <dir>`  Change directory (relative to root)\n" +
		"`/pwd`  Show current directory\n" +
		"`/ls`  List projects in root directory\n\n" +
		"**Git:**\n" +
		"`/git <args>`  Run git command\n" +
		"`/diff`  Show current changes\n" +
		"`/commit <msg>`  Quick commit\n" +
		"`/push`  Quick push\n" +
		"`/undo`  Discard uncommitted changes\n" +
		"`/stash [pop]`  Stash/restore changes\n\n" +
		"**Session:**\n" +
		"`/new`  Start new Claude session\n" +
		"`/sessions`  List session history\n" +
		"`/switch <id>`  Switch to session\n\n" +
		"**Control:**\n" +
		"`/kill`  Terminate current execution\n" +
		"`/model <name>`  Switch Claude model\n" +
		"`/yolo`  Enable unrestricted mode\n" +
		"`/safe`  Restore safe mode\n" +
		"`/last`  Show last Claude output\n" +
		"`/summary`  Summarize last output\n\n" +
		"**System:**\n" +
		"`/sh <cmd>`  Run shell command (via Claude)\n" +
		"`/file <path>`  Send project file to chat\n\n" +
		"**Docs:**\n" +
		"`/doc push <path>`  Push Markdown to Feishu doc\n" +
		"`/doc pull <path>`  Pull shared doc to project\n" +
		"`/doc bind <path> <url|id>`  Bind file to Feishu doc\n" +
		"`/doc unbind <path>`  Unbind\n" +
		"`/doc list`  List bindings\n\n" +
		"Any other message is sent directly to Claude as a prompt."
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "DevBot Help", Content: md})
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

	var queuePending int
	if r.queue != nil {
		queuePending = r.queue.PendingCount(chatID)
	}

	lastExec := r.executor.LastExecDuration().Truncate(time.Millisecond)
	lastExecStr := "-"
	if r.executor.ExecCount() > 0 {
		lastExecStr = lastExec.String()
	}

	md := fmt.Sprintf("**WorkDir:**  %s\n**Session:**  %s\n**Model:**    %s\n**Mode:**     %s\n**Running:**  %v\n**Execs:**    %d\n**LastExec:** %s\n**Queued:**   %d\n**Uptime:**   %s",
		session.WorkDir,
		session.ClaudeSessionID,
		session.Model,
		mode,
		r.executor.IsRunning(),
		r.executor.ExecCount(),
		lastExecStr,
		queuePending,
		uptime,
	)
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "Status", Content: md})
}

func (r *Router) cmdPwd(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	r.sender.SendText(ctx, chatID, session.WorkDir)
}

func (r *Router) cmdLs(ctx context.Context, chatID string) {
	root := r.store.WorkRoot()
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
		r.sender.SendText(ctx, chatID, "Current root: "+r.store.WorkRoot())
		return
	}
	if !filepath.IsAbs(args) {
		r.sender.SendText(ctx, chatID, "Root must be an absolute path.")
		return
	}
	cleaned := filepath.Clean(args)
	if cleaned == "/" || strings.HasPrefix(cleaned, "/etc") ||
		strings.HasPrefix(cleaned, "/var") || strings.HasPrefix(cleaned, "/usr") ||
		strings.HasPrefix(cleaned, "/sys") || strings.HasPrefix(cleaned, "/proc") {
		r.sender.SendText(ctx, chatID, "Cannot set root to a system directory.")
		return
	}
	info, err := os.Stat(args)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Directory not found: %s", args))
		return
	}
	if !info.IsDir() {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Not a directory: %s", args))
		return
	}
	r.store.SetWorkRoot(args)
	r.save()
	r.sender.SendText(ctx, chatID, "Root set to: "+args)
}

func (r *Router) cmdCd(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "Usage: /cd <directory>")
		return
	}
	r.getSession(chatID) // ensure session exists
	root := r.store.WorkRoot()

	var target string
	if filepath.IsAbs(args) {
		target = args
	} else {
		target = filepath.Join(root, args)
	}
	target = filepath.Clean(target)

	// Prevent path traversal outside work root
	if !strings.HasPrefix(target, root) {
		r.sender.SendText(ctx, chatID, "Cannot cd outside of work root: "+root)
		return
	}

	if _, err := os.Stat(target); err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Directory not found: %s", target))
		return
	}
	r.store.UpdateSession(chatID, func(s *Session) {
		s.WorkDir = target
	})
	r.save()
	r.sender.SendText(ctx, chatID, "Changed to: "+target)
}

func (r *Router) cmdNewSession(ctx context.Context, chatID string) {
	r.getSession(chatID) // ensure session exists
	r.store.UpdateSession(chatID, func(s *Session) {
		if s.ClaudeSessionID != "" {
			s.History = append(s.History, s.ClaudeSessionID)
		}
		s.ClaudeSessionID = ""
		s.LastOutput = ""
	})
	r.save()
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
		lines = append(lines, fmt.Sprintf("  **\\* %s** (current)", session.ClaudeSessionID))
	}
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "Sessions", Content: strings.Join(lines, "\n")})
}

func (r *Router) cmdSwitch(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "Usage: /switch <session-id>")
		return
	}
	r.getSession(chatID) // ensure session exists
	r.store.UpdateSession(chatID, func(s *Session) {
		if s.ClaudeSessionID != "" {
			s.History = append(s.History, s.ClaudeSessionID)
		}
		s.ClaudeSessionID = args
		s.LastOutput = ""
	})
	r.save()
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
	r.getSession(chatID) // ensure session exists
	r.store.UpdateSession(chatID, func(s *Session) {
		s.Model = args
	})
	r.save()
	r.sender.SendText(ctx, chatID, "Model set to: "+args)
}

func (r *Router) cmdYolo(ctx context.Context, chatID string) {
	r.getSession(chatID) // ensure session exists
	r.store.UpdateSession(chatID, func(s *Session) {
		s.PermissionMode = "yolo"
	})
	r.save()
	r.sender.SendText(ctx, chatID, "YOLO mode enabled. Claude will execute without restrictions.")
}

func (r *Router) cmdSafe(ctx context.Context, chatID string) {
	r.getSession(chatID) // ensure session exists
	r.store.UpdateSession(chatID, func(s *Session) {
		s.PermissionMode = "safe"
	})
	r.save()
	r.sender.SendText(ctx, chatID, "Safe mode restored.")
}

func (r *Router) cmdLast(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	if session.LastOutput == "" {
		r.sender.SendText(ctx, chatID, "No previous output.")
		return
	}
	r.sender.SendCard(ctx, chatID, CardMsg{Content: session.LastOutput})
}

func (r *Router) cmdSummary(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	if session.LastOutput == "" {
		r.sender.SendText(ctx, chatID, "No previous output to summarize.")
		return
	}
	prompt := "Please summarize the following output concisely:\n\n" + session.LastOutput
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdCommit(ctx context.Context, chatID, msg string) {
	if msg == "" {
		r.sender.SendText(ctx, chatID, "Usage: /commit <message>")
		return
	}
	r.getSession(chatID) // ensure session exists
	prompt := fmt.Sprintf("Stage tracked file changes with `git add -u` (do NOT use `git add -A` to avoid staging untracked files), then commit with the message: %s\nOnly show the command output, no explanation.", msg)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdGit(ctx context.Context, chatID, args string) {
	r.getSession(chatID) // ensure session exists
	prompt := fmt.Sprintf("Run `git %s` in the current directory and return the output. Only show the command output, no explanation.", args)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdSh(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "Usage: /sh <command>")
		return
	}
	r.getSession(chatID) // ensure session exists
	prompt := fmt.Sprintf("Run `%s` in the current directory and return the output. Only show the command output, no explanation.", args)
	r.execClaudeQueued(ctx, chatID, prompt)
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
	r.sender.SendCard(ctx, chatID, CardMsg{Title: filepath.Base(target), Content: "```\n" + string(data) + "\n```"})
}

func findFile(workDir, query string) string {
	if filepath.IsAbs(query) {
		return "" // Don't allow absolute paths
	}
	exact := filepath.Join(workDir, query)
	if _, err := os.Stat(exact); err == nil {
		return exact
	}
	query = strings.ToLower(query)
	var match string
	count := 0
	filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		count++
		if count > 10000 {
			return filepath.SkipAll
		}
		if strings.Contains(strings.ToLower(info.Name()), query) {
			match = path
			return filepath.SkipAll
		}
		return nil
	})
	return match
}

// findDocBinding looks up a doc binding by fuzzy path match. It tries:
// 1. Exact path (joined with workDir)
// 2. Case-insensitive substring match on binding keys
// Returns (filePath, docID) or ("", "") if not found.
func (r *Router) findDocBinding(workDir, query string) (string, string) {
	bindings := r.store.DocBindings()

	// Try exact path first
	if !filepath.IsAbs(query) {
		exact := filepath.Clean(filepath.Join(workDir, query))
		if docID, ok := bindings[exact]; ok {
			return exact, docID
		}
	}

	// Fuzzy: case-insensitive substring match on binding keys
	queryLower := strings.ToLower(query)
	for path, docID := range bindings {
		if strings.Contains(strings.ToLower(filepath.Base(path)), queryLower) {
			return path, docID
		}
	}
	return "", ""
}

// resolveFilePath resolves a user-supplied path to an absolute path within
// the work directory. It tries exact match first, then falls back to fuzzy
// matching via findFile. Returns the resolved path or empty string if not found.
func resolveFilePath(workDir, query string) string {
	if filepath.IsAbs(query) {
		return "" // Reject absolute paths
	}
	exact := filepath.Join(workDir, query)
	if _, err := os.Stat(exact); err == nil {
		return filepath.Clean(exact)
	}
	return findFile(workDir, query)
}

func (r *Router) cmdDoc(ctx context.Context, chatID, args string) {
	parts := strings.SplitN(args, " ", 2)
	sub := ""
	subArgs := ""
	if len(parts) > 0 {
		sub = strings.ToLower(parts[0])
	}
	if len(parts) > 1 {
		subArgs = strings.TrimSpace(parts[1])
	}

	switch sub {
	case "push":
		r.cmdDocPush(ctx, chatID, subArgs)
	case "pull":
		r.cmdDocPull(ctx, chatID, subArgs)
	case "bind":
		r.cmdDocBind(ctx, chatID, subArgs)
	case "unbind":
		r.cmdDocUnbind(ctx, chatID, subArgs)
	case "list":
		r.cmdDocList(ctx, chatID)
	case "":
		r.sender.SendText(ctx, chatID, "Usage: /doc push|pull|bind|unbind|list")
	default:
		r.sender.SendText(ctx, chatID, "Unknown doc subcommand. Usage: /doc push|pull|bind|unbind|list")
	}
}

func (r *Router) cmdDocPush(ctx context.Context, chatID, args string) {
	if r.docSyncer == nil {
		r.sender.SendText(ctx, chatID, "Doc sync not configured.")
		return
	}
	if args == "" {
		r.sender.SendText(ctx, chatID, "Usage: /doc push <path>")
		return
	}

	session := r.getSession(chatID)
	filePath := resolveFilePath(session.WorkDir, args)
	if filePath == "" {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("File not found: %s", args))
		return
	}

	root := r.store.WorkRoot()
	if !strings.HasPrefix(filePath, root) {
		r.sender.SendText(ctx, chatID, "Cannot access files outside work root: "+root)
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Error reading file: %v", err))
		return
	}

	title := filepath.Base(filePath)
	content := string(data)

	docID, docURL, err := r.docSyncer.CreateAndPushDoc(ctx, title, content)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Error pushing doc: %v", err))
		return
	}

	r.store.SetDocBinding(filePath, docID)
	r.save()

	md := fmt.Sprintf("**ID:** %s\n**URL:** [%s](%s)", docID, docURL, docURL)
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "Doc Pushed", Content: md})
}

func (r *Router) cmdDocPull(ctx context.Context, chatID, args string) {
	if r.docSyncer == nil {
		r.sender.SendText(ctx, chatID, "Doc sync not configured.")
		return
	}
	if args == "" {
		r.sender.SendText(ctx, chatID, "Usage: /doc pull <path>")
		return
	}

	session := r.getSession(chatID)
	filePath, docID := r.findDocBinding(session.WorkDir, args)
	if docID == "" {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("No binding found for %s. Use /doc bind first.", args))
		return
	}

	root := r.store.WorkRoot()
	if !strings.HasPrefix(filePath, root) {
		r.sender.SendText(ctx, chatID, "Cannot access files outside work root: "+root)
		return
	}

	content, err := r.docSyncer.PullDocContent(ctx, docID)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Error pulling doc: %v", err))
		return
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Error writing file: %v", err))
		return
	}

	r.sender.SendText(ctx, chatID, fmt.Sprintf("Pulled doc %s to %s", docID, args))
}

func (r *Router) cmdDocBind(ctx context.Context, chatID, args string) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		r.sender.SendText(ctx, chatID, "Usage: /doc bind <path> <docURL or docID>")
		return
	}

	session := r.getSession(chatID)
	filePath := resolveFilePath(session.WorkDir, parts[0])
	if filePath == "" {
		// For bind, allow binding a not-yet-existing file (exact path)
		filePath = filepath.Clean(filepath.Join(session.WorkDir, parts[0]))
	}

	root := r.store.WorkRoot()
	if !strings.HasPrefix(filePath, root) {
		r.sender.SendText(ctx, chatID, "Cannot access files outside work root: "+root)
		return
	}

	docID := ParseDocID(parts[1])
	r.store.SetDocBinding(filePath, docID)
	r.save()

	r.sender.SendText(ctx, chatID, fmt.Sprintf("Bound %s -> %s", parts[0], docID))
}

func (r *Router) cmdDocUnbind(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "Usage: /doc unbind <path>")
		return
	}

	session := r.getSession(chatID)
	filePath, docID := r.findDocBinding(session.WorkDir, args)
	if docID == "" {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("No binding found for %s", args))
		return
	}

	r.store.RemoveDocBinding(filePath)
	r.save()

	r.sender.SendText(ctx, chatID, fmt.Sprintf("Unbound %s", args))
}

func (r *Router) cmdDocList(ctx context.Context, chatID string) {
	bindings := r.store.DocBindings()
	if len(bindings) == 0 {
		r.sender.SendText(ctx, chatID, "No bindings.")
		return
	}

	var lines []string
	for path, docID := range bindings {
		lines = append(lines, fmt.Sprintf("**%s** -> %s", path, docID))
	}
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "Doc Bindings", Content: strings.Join(lines, "\n")})
}

func (r *Router) RouteImage(ctx context.Context, chatID, userID string, imageData []byte, fileName string) {
	if !r.allowedUsers[userID] {
		return
	}

	session := r.getSession(chatID)

	// Save image to work directory
	imgDir := filepath.Join(session.WorkDir, ".devbot-images")
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Failed to create image directory: %v", err))
		return
	}
	imgPath := filepath.Join(imgDir, filepath.Base(fileName))
	if err := os.WriteFile(imgPath, imageData, 0644); err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Failed to save image: %v", err))
		return
	}

	r.sender.SendText(ctx, chatID, fmt.Sprintf("Image saved to: %s", imgPath))
	prompt := fmt.Sprintf("User sent an image, saved to: %s. Describe or process this image as needed.", imgPath)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) RouteFile(ctx context.Context, chatID, userID, fileName string, fileData []byte) {
	if !r.allowedUsers[userID] {
		return
	}

	session := r.getSession(chatID)

	// Save file to work directory (use Base to prevent path traversal)
	filePath := filepath.Join(session.WorkDir, filepath.Base(fileName))
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Failed to save file: %v", err))
		return
	}

	r.sender.SendText(ctx, chatID, fmt.Sprintf("File saved to: %s", filePath))
	prompt := fmt.Sprintf("User sent a file '%s', saved to: %s. Examine or process this file as needed.", fileName, filePath)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) RouteDocShare(ctx context.Context, chatID, userID, docID string) {
	if !r.allowedUsers[userID] {
		return
	}
	r.sender.SendText(ctx, chatID, fmt.Sprintf("Detected Feishu doc: %s\nUse /doc bind <path> %s to bind it to a local file.\nOr /doc pull <path> if already bound.", docID, docID))
}

func (r *Router) handlePrompt(ctx context.Context, chatID, text string) {
	r.getSession(chatID) // ensure session exists
	r.execClaudeQueued(ctx, chatID, text)
}

func (r *Router) execClaudeQueued(ctx context.Context, chatID string, prompt string) {
	if r.queue != nil {
		pending := r.queue.PendingCount(chatID)
		if pending > 0 {
			r.sender.SendCard(ctx, chatID, CardMsg{Title: fmt.Sprintf("Queued (position %d)", pending+1), Template: "blue"})
		}
		if err := r.queue.Enqueue(chatID, func() {
			r.execClaude(r.ctx, chatID, prompt)
		}); err != nil {
			r.sender.SendText(ctx, chatID, "Queue is full, please try again later.")
		}
	} else {
		r.execClaude(ctx, chatID, prompt)
	}
}

func (r *Router) execClaude(ctx context.Context, chatID string, prompt string) {
	workDir, sessionID, permMode, model := r.store.SessionExecParams(chatID)
	if permMode == "" {
		permMode = "safe"
	}

	startTime := time.Now()
	var lastSendTime time.Time

	onProgress := func(text string) {
		now := time.Now()
		elapsed := now.Sub(startTime)
		sinceLast := now.Sub(lastSendTime)

		// Only send progress after 5 seconds, then every 10 seconds
		if elapsed < 5*time.Second {
			return
		}
		if sinceLast < 10*time.Second {
			return
		}

		lastSendTime = now
		display := text
		if len(display) > 1000 {
			display = "..." + display[len(display)-1000:]
		}
		title := fmt.Sprintf("Executing... (%ds)", int(elapsed.Seconds()))
		r.sender.SendCard(ctx, chatID, CardMsg{Title: title, Content: display, Template: "blue"})
	}

	result, err := r.executor.ExecStream(ctx, prompt, workDir, sessionID, permMode, model, onProgress)
	if err != nil {
		r.sender.SendCard(ctx, chatID, CardMsg{Title: "Error", Content: fmt.Sprintf("%v", err), Template: "red"})
		return
	}

	r.store.UpdateSession(chatID, func(s *Session) {
		s.LastOutput = result.Output
		if result.SessionID != "" {
			s.ClaudeSessionID = result.SessionID
		}
	})
	r.save()

	output := result.Output
	if output == "" {
		output = "(empty response)"
	}
	if result.IsPermissionDenial {
		r.sender.SendCard(ctx, chatID, CardMsg{Title: "Claude 想确认", Content: output, Template: "purple"})
		return
	}
	r.sender.SendCard(ctx, chatID, CardMsg{Content: output})
}
