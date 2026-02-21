package bot

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"devbot/internal/version"
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
	case "/version":
		r.cmdVersion(ctx, chatID)
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
		r.cmdUndo(ctx, chatID)
	case "/stash":
		if args == "" {
			r.cmdGit(ctx, chatID, "stash")
		} else {
			r.cmdGit(ctx, chatID, "stash "+args)
		}
	case "/log":
		r.cmdLog(ctx, chatID, args)
	case "/branch":
		r.cmdBranch(ctx, chatID, args)
	case "/cancel":
		r.cmdKill(ctx, chatID)
	case "/retry":
		r.cmdRetry(ctx, chatID)
	case "/info":
		r.cmdInfo(ctx, chatID)
	case "/grep":
		r.cmdGrep(ctx, chatID, args)
	case "/pr":
		r.cmdPR(ctx, chatID, args)
	case "/sh":
		r.cmdSh(ctx, chatID, args)
	case "/file":
		r.cmdFile(ctx, chatID, args)
	case "/doc":
		r.cmdDoc(ctx, chatID, args)
	default:
		r.sender.SendText(ctx, chatID, fmt.Sprintf("未知命令: %s\n\n使用 /help 查看所有可用命令。", cmd))
	}
}

func (r *Router) getSession(chatID string) Session {
	return r.store.GetSession(chatID, r.store.WorkRoot(), r.executor.Model())
}

func (r *Router) cmdHelp(ctx context.Context, chatID string) {
	md := "**🗺 导航:**\n" +
		"`/info`  快速概览（目录、分支、变更、状态）\n" +
		"`/root [path]`  查看/设置根工作目录\n" +
		"`/cd <dir>`  切换项目目录（支持相对路径）\n" +
		"`/pwd`  显示当前目录\n" +
		"`/ls`  列出根目录下的项目\n\n" +
		"**🤖 Claude 对话:**\n" +
		"`/status`  查看详细状态（含 git 信息）\n" +
		"`/new`  开启新对话（保留当前会话到历史）\n" +
		"`/kill`  终止正在执行的任务\n" +
		"`/cancel`  同 /kill，终止当前任务\n" +
		"`/retry`  重试上一条发给 Claude 的消息\n" +
		"`/last`  显示上次输出\n" +
		"`/summary`  让 Claude 总结上次输出\n" +
		"`/model [name]`  查看/切换模型（haiku/sonnet/opus）\n" +
		"`/yolo`  开启无限制模式（Claude 可执行所有操作）\n" +
		"`/safe`  恢复安全模式\n\n" +
		"**🔀 历史会话:**\n" +
		"`/sessions`  查看历史会话列表\n" +
		"`/switch <id>`  切换到指定历史会话\n\n" +
		"**🔧 Git:**\n" +
		"`/diff`  查看当前变更\n" +
		"`/log [n]`  查看提交历史（默认最近 20 条）\n" +
		"`/branch [name]`  查看分支列表或切换/创建分支\n" +
		"`/commit [msg]`  提交（不填消息则 Claude 自动生成）\n" +
		"`/push`  推送到远程\n" +
		"`/pr [title]`  创建 Pull Request\n" +
		"`/undo`  ⚠️ 撤销所有未提交的更改（无变更时提示而非执行）\n" +
		"`/stash [pop]`  暂存/恢复更改\n" +
		"`/git <args>`  执行任意 git 命令\n\n" +
		"**📁 文件与搜索:**\n" +
		"`/grep <pattern>`  在代码中搜索关键词\n" +
		"`/file <path>`  查看项目文件内容\n" +
		"`/sh <cmd>`  通过 Claude 执行 Shell 命令\n\n" +
		"**📄 飞书文档同步:**\n" +
		"`/doc push <path>`  将 Markdown 文件推送到飞书文档\n" +
		"`/doc pull <path>`  将飞书文档内容拉取到本地文件\n" +
		"`/doc bind <path> <url|id>`  绑定本地文件到飞书文档\n" +
		"`/doc unbind <path>`  解除绑定\n" +
		"`/doc list`  查看所有绑定关系\n\n" +
		"**💬 其他:**\n" +
		"`/ping`  检查机器人是否在线\n" +
		"`/version`  显示版本信息（版本号、Commit、构建时间）\n" +
		"`/help`  显示此帮助\n\n" +
		"直接发送文字即可与 Claude 对话，也可发送图片或文件。"
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "DevBot 使用指南", Content: md})
}

func (r *Router) cmdPing(ctx context.Context, chatID string) {
	uptime := time.Since(r.startTime).Truncate(time.Second)
	r.sender.SendText(ctx, chatID, fmt.Sprintf("pong ✓ (已运行 %s)", uptime))
}

func (r *Router) cmdVersion(ctx context.Context, chatID string) {
	r.sender.SendText(ctx, chatID, version.String())
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

	runningStr := "空闲"
	if r.executor.IsRunning() {
		runningStr = "执行中..."
	}
	sessionStr := session.ClaudeSessionID
	if sessionStr == "" {
		sessionStr = "（新会话）"
	}
	branch := gitBranch(session.WorkDir)
	branchStr := branch
	if branchStr == "" {
		branchStr = "（非 git 目录）"
	}
	changes := gitStatusSummary(session.WorkDir)
	if changes == "" {
		changes = "（非 git 目录）"
	}
	md := fmt.Sprintf("**工作目录:** `%s`\n**Git 分支:**  %s\n**工作区:**    %s\n**会话 ID:**   `%s`\n**模型:**      %s\n**模式:**      %s\n**状态:**      %s\n**执行次数:** %d\n**上次耗时:** %s\n**待执行队列:** %d\n**运行时长:** %s",
		session.WorkDir,
		branchStr,
		changes,
		sessionStr,
		session.Model,
		mode,
		runningStr,
		r.executor.ExecCount(),
		lastExecStr,
		queuePending,
		uptime,
	)
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "当前状态", Content: md})
}

func (r *Router) cmdPwd(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	r.sender.SendText(ctx, chatID, session.WorkDir)
}

func (r *Router) cmdLs(ctx context.Context, chatID string) {
	root := r.store.WorkRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("读取目录出错: %v", err))
		return
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("根目录 %s 下暂无项目目录。\n使用 /cd <目录名> 切换到指定目录。", root))
		return
	}
	r.sender.SendCard(ctx, chatID, CardMsg{
		Title:   fmt.Sprintf("项目列表 (%s)", root),
		Content: strings.Join(dirs, "\n"),
	})
}

func (r *Router) cmdRoot(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "当前根目录: "+r.store.WorkRoot())
		return
	}
	if !filepath.IsAbs(args) {
		r.sender.SendText(ctx, chatID, "根目录必须是绝对路径，例如: /home/user/projects")
		return
	}
	cleaned := filepath.Clean(args)
	if cleaned == "/" || strings.HasPrefix(cleaned, "/etc") ||
		strings.HasPrefix(cleaned, "/var") || strings.HasPrefix(cleaned, "/usr") ||
		strings.HasPrefix(cleaned, "/sys") || strings.HasPrefix(cleaned, "/proc") {
		r.sender.SendText(ctx, chatID, "不允许将系统目录设为根目录。")
		return
	}
	info, err := os.Stat(args)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("目录不存在: %s", args))
		return
	}
	if !info.IsDir() {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("不是目录: %s", args))
		return
	}
	r.store.SetWorkRoot(args)
	r.save()
	r.sender.SendText(ctx, chatID, fmt.Sprintf("✓ 根目录已设置为: %s", args))
}

func (r *Router) cmdCd(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "用法: /cd <目录名>\n示例: /cd myproject\n\n使用 /ls 查看可用项目列表。")
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
	if !underRoot(root, target) {
		r.sender.SendText(ctx, chatID, "不允许切换到工作根目录以外的路径: "+root)
		return
	}

	if _, err := os.Stat(target); err != nil {
		// Show available subdirectories to help user navigate
		msg := fmt.Sprintf("目录不存在: %s", target)
		if entries, readErr := os.ReadDir(root); readErr == nil {
			var dirs []string
			for _, e := range entries {
				if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
					dirs = append(dirs, e.Name())
				}
			}
			if len(dirs) > 0 {
				msg += "\n\n可用目录:\n" + strings.Join(dirs, "  /  ")
			}
		}
		r.sender.SendText(ctx, chatID, msg)
		return
	}
	r.store.UpdateSession(chatID, func(s *Session) {
		// Save current dir's session before switching
		if s.DirSessions == nil {
			s.DirSessions = make(map[string]string)
		}
		if s.ClaudeSessionID != "" && s.WorkDir != "" {
			s.DirSessions[s.WorkDir] = s.ClaudeSessionID
		}
		// Restore session for the new directory (empty string = new session)
		s.ClaudeSessionID = s.DirSessions[target]
		s.WorkDir = target
		s.LastOutput = ""
	})
	r.save()
	r.sender.SendText(ctx, chatID, fmt.Sprintf("✓ 已切换到: %s", target))
}

func (r *Router) cmdNewSession(ctx context.Context, chatID string) {
	r.getSession(chatID) // ensure session exists
	var oldSessionID string
	r.store.UpdateSession(chatID, func(s *Session) {
		oldSessionID = s.ClaudeSessionID
		if s.ClaudeSessionID != "" {
			s.History = append(s.History, s.ClaudeSessionID)
		}
		s.ClaudeSessionID = ""
		s.LastOutput = ""
	})
	r.save()
	if oldSessionID != "" {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("已开启新对话。旧会话 %s 已保存到历史，可用 /sessions 查看或 /switch 恢复。", oldSessionID))
	} else {
		r.sender.SendText(ctx, chatID, "已开启新对话。")
	}
}

func (r *Router) cmdSessions(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	if len(session.History) == 0 && session.ClaudeSessionID == "" {
		r.sender.SendText(ctx, chatID, "暂无历史会话。发送消息后会自动创建会话。")
		return
	}
	var lines []string
	for i, id := range session.History {
		lines = append(lines, fmt.Sprintf("  `%d`: %s  （使用 `/switch %d` 恢复）", i, id, i))
	}
	if session.ClaudeSessionID != "" {
		lines = append(lines, fmt.Sprintf("\n**当前:** `%s`", session.ClaudeSessionID))
	}
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "历史会话", Content: strings.Join(lines, "\n")})
}

func (r *Router) cmdSwitch(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "用法: /switch <序号或会话ID>\n\n使用 /sessions 查看可用会话列表。")
		return
	}
	r.getSession(chatID) // ensure session exists

	// Support switching by index (from /sessions list)
	targetID := args
	if idx, err := fmt.Sscanf(args, "%d", new(int)); err == nil && idx == 1 {
		var idxVal int
		fmt.Sscanf(args, "%d", &idxVal)
		session := r.getSession(chatID)
		if idxVal >= 0 && idxVal < len(session.History) {
			targetID = session.History[idxVal]
		} else {
			r.sender.SendText(ctx, chatID, fmt.Sprintf("序号 %d 不存在，请用 /sessions 查看有效序号。", idxVal))
			return
		}
	}

	r.store.UpdateSession(chatID, func(s *Session) {
		if s.ClaudeSessionID != "" {
			s.History = append(s.History, s.ClaudeSessionID)
		}
		s.ClaudeSessionID = targetID
		s.LastOutput = ""
	})
	r.save()
	r.sender.SendText(ctx, chatID, fmt.Sprintf("✓ 已切换到会话: %s", targetID))
}

func (r *Router) cmdKill(ctx context.Context, chatID string) {
	if err := r.executor.Kill(); err != nil {
		r.sender.SendText(ctx, chatID, "当前没有正在执行的任务。")
		return
	}
	r.sender.SendText(ctx, chatID, "✓ 任务已终止。")
}

func (r *Router) cmdModel(ctx context.Context, chatID, args string) {
	if args == "" {
		session := r.getSession(chatID)
		current := session.Model
		if current == "" {
			current = r.executor.Model()
		}
		md := fmt.Sprintf("**当前模型:** `%s`\n\n**可选模型:**\n", current) +
			"- `haiku`  最快，适合简单任务和代码补全\n" +
			"- `sonnet`  均衡，推荐日常使用\n" +
			"- `opus`  最强，适合复杂推理和长任务\n\n" +
			"使用 `/model <名称>` 切换，例如 `/model opus`"
		r.sender.SendCard(ctx, chatID, CardMsg{Title: "模型设置", Content: md})
		return
	}
	r.getSession(chatID) // ensure session exists
	r.store.UpdateSession(chatID, func(s *Session) {
		s.Model = args
	})
	r.save()
	r.sender.SendText(ctx, chatID, fmt.Sprintf("✓ 模型已切换为: %s", args))
}

func (r *Router) cmdYolo(ctx context.Context, chatID string) {
	r.getSession(chatID) // ensure session exists
	r.store.UpdateSession(chatID, func(s *Session) {
		s.PermissionMode = "yolo"
	})
	r.save()
	md := "⚠️ **已开启无限制模式（YOLO）**\n\n" +
		"Claude 现在可以执行所有操作，包括：\n" +
		"- 运行任意 Shell 命令\n" +
		"- 修改、删除文件\n" +
		"- 访问网络\n\n" +
		"使用 `/safe` 恢复安全模式。"
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "⚠️ 无限制模式已开启", Content: md, Template: "orange"})
}

func (r *Router) cmdSafe(ctx context.Context, chatID string) {
	r.getSession(chatID) // ensure session exists
	r.store.UpdateSession(chatID, func(s *Session) {
		s.PermissionMode = "safe"
	})
	r.save()
	r.sender.SendText(ctx, chatID, "✓ 已恢复安全模式，Claude 的操作需要确认。")
}

func (r *Router) cmdLast(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	if session.LastOutput == "" {
		r.sender.SendText(ctx, chatID, "暂无历史输出，请先发送消息给 Claude。")
		return
	}
	r.sender.SendCard(ctx, chatID, CardMsg{Content: session.LastOutput})
}

func (r *Router) cmdSummary(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	if session.LastOutput == "" {
		r.sender.SendText(ctx, chatID, "暂无可总结的输出，请先发送消息给 Claude。")
		return
	}
	prompt := "Please summarize the following output concisely:\n\n" + session.LastOutput
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdCommit(ctx context.Context, chatID, msg string) {
	r.getSession(chatID) // ensure session exists
	var prompt string
	if msg == "" {
		prompt = "Stage tracked file changes with `git add -u` (do NOT use `git add -A` to avoid staging untracked files), then write a concise commit message based on the changes (`git diff --cached`), and commit. Only show the final commit output, no explanation."
	} else {
		prompt = fmt.Sprintf("Stage tracked file changes with `git add -u` (do NOT use `git add -A` to avoid staging untracked files), then commit with the message: %s\nOnly show the command output, no explanation.", msg)
	}
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdGit(ctx context.Context, chatID, args string) {
	r.getSession(chatID) // ensure session exists
	prompt := fmt.Sprintf("Run `git %s` in the current directory and return the output. Only show the command output, no explanation.", args)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdUndo(ctx context.Context, chatID string) {
	r.getSession(chatID) // ensure session exists
	changes := gitStatusSummary(r.store.GetSession(chatID, r.store.WorkRoot(), r.executor.Model()).WorkDir)
	if changes == "无变更" || changes == "" {
		r.sender.SendText(ctx, chatID, "当前没有未提交的更改，无需撤销。")
		return
	}
	prompt := fmt.Sprintf("⚠️ 即将撤销所有未提交的更改（%s）。运行 `git checkout .` 撤销工作目录变更（已暂存的变更不受影响）。只输出命令结果，不要解释。", changes)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdLog(ctx context.Context, chatID, args string) {
	r.getSession(chatID) // ensure session exists
	count := "20"
	if args != "" {
		count = args
	}
	prompt := fmt.Sprintf("Run `git log --oneline -%s` in the current directory and return the output. Only show the command output, no explanation.", count)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdBranch(ctx context.Context, chatID, args string) {
	r.getSession(chatID) // ensure session exists
	if args == "" {
		prompt := "Run `git branch -v` in the current directory and return the output, showing which branch is current. Only show the command output, no explanation."
		r.execClaudeQueued(ctx, chatID, prompt)
		return
	}
	// Create new branch or switch to existing
	prompt := fmt.Sprintf("Run `git checkout -b %s 2>/dev/null || git checkout %s` in the current directory and return the output. Only show the command output, no explanation.", args, args)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdRetry(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	if session.LastPrompt == "" {
		r.sender.SendText(ctx, chatID, "没有可重试的请求。")
		return
	}
	r.sender.SendText(ctx, chatID, fmt.Sprintf("重试: %s", session.LastPrompt))
	r.execClaudeQueued(ctx, chatID, session.LastPrompt)
}

func (r *Router) cmdInfo(ctx context.Context, chatID string) {
	session := r.getSession(chatID)
	mode := session.PermissionMode
	if mode == "" {
		mode = "safe"
	}
	branch := gitBranch(session.WorkDir)
	if branch == "" {
		branch = "（非 git 目录）"
	}
	changes := gitStatusSummary(session.WorkDir)
	if changes == "" {
		changes = "（非 git 目录）"
	}
	runningStr := "空闲"
	if r.executor.IsRunning() {
		runningStr = "执行中..."
	}
	md := fmt.Sprintf("📂 `%s`\n🌿 %s | 📝 %s\n🤖 %s | 🔒 %s | ⚡ %s",
		session.WorkDir, branch, changes, session.Model, mode, runningStr)
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "当前概览", Content: md})
}

func (r *Router) cmdGrep(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "用法: /grep <关键词>\n示例: /grep TODO\n示例: /grep func main")
		return
	}
	r.getSession(chatID) // ensure session exists
	prompt := fmt.Sprintf("Run `grep -rn --include='*.go' --include='*.ts' --include='*.py' --include='*.js' -l %q .` in the current directory, then show the top matching lines. Only show the command output, no explanation.", args)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdPR(ctx context.Context, chatID, args string) {
	r.getSession(chatID) // ensure session exists
	var prompt string
	if args == "" {
		prompt = "Create a pull request using `gh pr create` with an auto-generated title and body based on the current branch changes. Only show the PR URL in the output, no extra explanation."
	} else {
		prompt = fmt.Sprintf("Create a pull request using `gh pr create --title %q` with an auto-generated body based on the current branch changes. Only show the PR URL in the output, no extra explanation.", args)
	}
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdSh(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "用法: /sh <命令>\n示例: /sh ls -la\n示例: /sh cat README.md")
		return
	}
	r.getSession(chatID) // ensure session exists
	prompt := fmt.Sprintf("Run `%s` in the current directory and return the output. Only show the command output, no explanation.", args)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) cmdFile(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "用法: /file <文件路径>\n示例: /file README.md\n示例: /file src/main.go")
		return
	}
	session := r.getSession(chatID)
	target := findFile(session.WorkDir, args)
	if target == "" {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("文件不存在: %s", args))
		return
	}
	data, err := os.ReadFile(target)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("读取文件出错: %v", err))
		return
	}
	r.sender.SendCard(ctx, chatID, CardMsg{Title: filepath.Base(target), Content: "```\n" + string(data) + "\n```"})
}

// gitBranch returns the current git branch name in workDir, or empty on error.
func gitBranch(workDir string) string {
	if workDir == "" {
		return ""
	}
	var out bytes.Buffer
	cmd := exec.Command("git", "-C", workDir, "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	branch := strings.TrimSpace(out.String())
	if branch == "HEAD" {
		return "" // detached HEAD — not useful to show
	}
	return branch
}

// gitStatusSummary returns a brief summary of working tree changes, or empty on error.
func gitStatusSummary(workDir string) string {
	if workDir == "" {
		return ""
	}
	var out bytes.Buffer
	cmd := exec.Command("git", "-C", workDir, "status", "--porcelain")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	output := strings.TrimSpace(out.String())
	if output == "" {
		return "无变更"
	}
	lines := strings.Split(output, "\n")
	return fmt.Sprintf("%d 个文件变更", len(lines))
}

// underRoot reports whether path is equal to root or is directly under it.
// It handles the edge case where root="/a/b" and path="/a/b2/..." would
// incorrectly pass a naive strings.HasPrefix check.
func underRoot(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
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
		r.sender.SendText(ctx, chatID, "用法: /doc <子命令>\n\n子命令: push | pull | bind | unbind | list\n示例: /doc push README.md")
	default:
		r.sender.SendText(ctx, chatID, fmt.Sprintf("未知的 doc 子命令: %s\n\n支持的子命令: push | pull | bind | unbind | list", sub))
	}
}

func (r *Router) cmdDocPush(ctx context.Context, chatID, args string) {
	if r.docSyncer == nil {
		r.sender.SendText(ctx, chatID, "飞书文档同步未配置，请联系管理员检查 API 配置。")
		return
	}
	if args == "" {
		r.sender.SendText(ctx, chatID, "用法: /doc push <文件路径>\n示例: /doc push README.md")
		return
	}

	session := r.getSession(chatID)
	filePath := resolveFilePath(session.WorkDir, args)
	if filePath == "" {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("文件不存在: %s", args))
		return
	}

	root := r.store.WorkRoot()
	if !underRoot(root, filePath) {
		r.sender.SendText(ctx, chatID, "不允许访问工作根目录以外的文件: "+root)
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("读取文件出错: %v", err))
		return
	}

	title := filepath.Base(filePath)
	content := string(data)

	docID, docURL, err := r.docSyncer.CreateAndPushDoc(ctx, title, content)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("推送文档出错: %v", err))
		return
	}

	r.store.SetDocBinding(filePath, docID)
	r.save()

	md := fmt.Sprintf("**文档 ID:** %s\n**链接:** [%s](%s)", docID, docURL, docURL)
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "✓ 文档已推送", Content: md})
}

func (r *Router) cmdDocPull(ctx context.Context, chatID, args string) {
	if r.docSyncer == nil {
		r.sender.SendText(ctx, chatID, "飞书文档同步未配置，请联系管理员检查 API 配置。")
		return
	}
	if args == "" {
		r.sender.SendText(ctx, chatID, "用法: /doc pull <文件路径>\n示例: /doc pull README.md\n\n需先用 /doc bind 绑定文件到飞书文档。")
		return
	}

	session := r.getSession(chatID)
	filePath, docID := r.findDocBinding(session.WorkDir, args)
	if docID == "" {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("未找到 %s 的绑定关系，请先用 /doc bind 绑定到飞书文档。", args))
		return
	}

	root := r.store.WorkRoot()
	if !underRoot(root, filePath) {
		r.sender.SendText(ctx, chatID, "不允许访问工作根目录以外的文件: "+root)
		return
	}

	content, err := r.docSyncer.PullDocContent(ctx, docID)
	if err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("拉取文档出错: %v", err))
		return
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("写入文件出错: %v", err))
		return
	}

	r.sender.SendText(ctx, chatID, fmt.Sprintf("✓ 文档已拉取到: %s", args))
}

func (r *Router) cmdDocBind(ctx context.Context, chatID, args string) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		r.sender.SendText(ctx, chatID, "用法: /doc bind <文件路径> <文档URL或ID>\n示例: /doc bind README.md https://example.feishu.cn/docx/xxx")
		return
	}

	session := r.getSession(chatID)
	filePath := resolveFilePath(session.WorkDir, parts[0])
	if filePath == "" {
		// For bind, allow binding a not-yet-existing file (exact path)
		filePath = filepath.Clean(filepath.Join(session.WorkDir, parts[0]))
	}

	root := r.store.WorkRoot()
	if !underRoot(root, filePath) {
		r.sender.SendText(ctx, chatID, "不允许访问工作根目录以外的文件: "+root)
		return
	}

	docID := ParseDocID(parts[1])
	r.store.SetDocBinding(filePath, docID)
	r.save()

	r.sender.SendText(ctx, chatID, fmt.Sprintf("✓ 已绑定: %s → %s", parts[0], docID))
}

func (r *Router) cmdDocUnbind(ctx context.Context, chatID, args string) {
	if args == "" {
		r.sender.SendText(ctx, chatID, "用法: /doc unbind <文件路径>\n示例: /doc unbind README.md")
		return
	}

	session := r.getSession(chatID)
	filePath, docID := r.findDocBinding(session.WorkDir, args)
	if docID == "" {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("未找到 %s 的绑定关系，使用 /doc list 查看已有绑定。", args))
		return
	}

	r.store.RemoveDocBinding(filePath)
	r.save()

	r.sender.SendText(ctx, chatID, fmt.Sprintf("✓ 已解除绑定: %s", args))
}

func (r *Router) cmdDocList(ctx context.Context, chatID string) {
	bindings := r.store.DocBindings()
	if len(bindings) == 0 {
		r.sender.SendText(ctx, chatID, "暂无绑定关系。使用 /doc bind <路径> <URL> 创建绑定。")
		return
	}

	var lines []string
	for path, docID := range bindings {
		lines = append(lines, fmt.Sprintf("**%s** -> %s", path, docID))
	}
	r.sender.SendCard(ctx, chatID, CardMsg{Title: "文档绑定列表", Content: strings.Join(lines, "\n")})
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
		r.sender.SendText(ctx, chatID, fmt.Sprintf("图片保存失败: %v", err))
		return
	}

	r.sender.SendText(ctx, chatID, fmt.Sprintf("✓ 图片已保存: %s", imgPath))
	prompt := fmt.Sprintf("用户发来了一张图片，已保存到: %s。请描述或处理这张图片。", imgPath)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) RouteTextWithImages(ctx context.Context, chatID, userID, text string, images []ImageAttachment) {
	if !r.allowedUsers[userID] {
		return
	}

	session := r.getSession(chatID)

	// Save all images to work directory
	imgDir := filepath.Join(session.WorkDir, ".devbot-images")
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		r.sender.SendText(ctx, chatID, fmt.Sprintf("Failed to create image directory: %v", err))
		return
	}

	var savedPaths []string
	for _, img := range images {
		imgPath := filepath.Join(imgDir, filepath.Base(img.FileName))
		if err := os.WriteFile(imgPath, img.Data, 0644); err != nil {
			log.Printf("router: failed to save image %s: %v", img.FileName, err)
			continue
		}
		savedPaths = append(savedPaths, imgPath)
	}

	// Build prompt combining text and image paths
	var prompt string
	if text != "" && len(savedPaths) > 0 {
		prompt = text + "\n\n附带图片路径: " + strings.Join(savedPaths, ", ")
	} else if text != "" {
		prompt = text
	} else if len(savedPaths) > 0 {
		prompt = fmt.Sprintf("用户发来了一张图片，已保存到: %s。请描述或处理这张图片。", savedPaths[0])
	} else {
		return
	}

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

	r.sender.SendText(ctx, chatID, fmt.Sprintf("✓ 文件已保存: %s", filePath))
	prompt := fmt.Sprintf("用户发来了文件 '%s'，已保存到: %s。请检查或处理这个文件。", fileName, filePath)
	r.execClaudeQueued(ctx, chatID, prompt)
}

func (r *Router) RouteDocShare(ctx context.Context, chatID, userID, docID string) {
	if !r.allowedUsers[userID] {
		return
	}
	r.sender.SendText(ctx, chatID, fmt.Sprintf("检测到飞书文档: %s\n\n- 使用 `/doc bind <本地路径> %s` 绑定到本地文件\n- 或使用 `/doc pull <路径>` 拉取内容（如已绑定）", docID, docID))
}

func (r *Router) handlePrompt(ctx context.Context, chatID, text string) {
	r.getSession(chatID) // ensure session exists
	// Save prompt before queuing so /retry is always available
	r.store.UpdateSession(chatID, func(s *Session) {
		s.LastPrompt = text
	})
	r.execClaudeQueued(ctx, chatID, text)
}

func (r *Router) execClaudeQueued(ctx context.Context, chatID string, prompt string) {
	if r.queue != nil {
		pending := r.queue.PendingCount(chatID)
		if pending > 0 {
			r.sender.SendCard(ctx, chatID, CardMsg{Title: fmt.Sprintf("已排队（第 %d 位）", pending+1), Content: "当前有任务正在执行，请稍候...", Template: "blue"})
		}
		if err := r.queue.Enqueue(chatID, func() {
			r.execClaude(r.ctx, chatID, prompt)
		}); err != nil {
			r.sender.SendText(ctx, chatID, "队列已满，请稍后再试。")
		}
	} else {
		r.execClaude(ctx, chatID, prompt)
	}
}

func (r *Router) execClaude(ctx context.Context, chatID string, prompt string) {
	r.sender.SendText(ctx, chatID, "执行中...")

	workDir, sessionID, permMode, model := r.store.SessionExecParams(chatID)
	if permMode == "" {
		permMode = "safe"
	}

	startTime := time.Now()
	var lastSendTime time.Time
	var lastProgressContent string

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
		display := strings.TrimSpace(text)
		runes := []rune(display)
		if len(runes) > 4000 {
			display = "（内容过长，仅显示最新部分）\n\n" + string(runes[len(runes)-4000:])
		}
		lastProgressContent = display
		r.sender.SendCard(ctx, chatID, CardMsg{Content: display})
	}

	result, err := r.executor.ExecStream(ctx, prompt, workDir, sessionID, permMode, model, onProgress)
	elapsed := time.Since(startTime).Truncate(time.Second)
	if err != nil {
		// Auto-recover: if Claude session no longer exists, clear it and retry without --resume
		if sessionID != "" && strings.Contains(err.Error(), "No conversation found with session ID") {
			log.Printf("router: session %s not found, clearing and retrying without resume (chat=%s)", sessionID, chatID)
			r.store.UpdateSession(chatID, func(s *Session) {
				s.History = append(s.History, s.ClaudeSessionID)
				s.ClaudeSessionID = ""
			})
			r.save()
			result, err = r.executor.ExecStream(ctx, prompt, workDir, "", permMode, model, onProgress)
			elapsed = time.Since(startTime).Truncate(time.Second)
		}
	}
	if err != nil {
		log.Printf("router: execClaude error chat=%s elapsed=%s: %v", chatID, elapsed, err)
		r.sender.SendCard(ctx, chatID, CardMsg{Title: fmt.Sprintf("执行出错（%s）", elapsed), Content: fmt.Sprintf("%v", err), Template: "red"})
		return
	}

	r.store.UpdateSession(chatID, func(s *Session) {
		s.LastOutput = result.Output
		if result.SessionID != "" {
			s.ClaudeSessionID = result.SessionID
			// Keep dir→session map in sync
			if s.DirSessions == nil {
				s.DirSessions = make(map[string]string)
			}
			if s.WorkDir != "" {
				s.DirSessions[s.WorkDir] = result.SessionID
			}
		}
	})
	r.save()

	output := result.Output
	if output == "" {
		output = "（无输出）"
	}
	output = strings.TrimSpace(output)
	if result.IsPermissionDenial {
		if output != lastProgressContent {
			r.sender.SendCard(ctx, chatID, CardMsg{Title: "Claude 需要确认", Content: output + "\n\n使用 `/yolo` 开启无限制模式以跳过确认。", Template: "purple"})
		}
		return
	}
	// Skip result card if identical to the last progress card
	if output != lastProgressContent {
		r.sender.SendCard(ctx, chatID, CardMsg{Content: output})
	}
	r.sender.SendText(ctx, chatID, fmt.Sprintf("✓ 完成（耗时 %s）", elapsed))
}
