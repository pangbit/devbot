# devbot — Feishu Claude Code Controller

Control Claude Code CLI remotely through Feishu (飞书) bot messages. Designed for mobile-first project development.

## Requirements

- Go 1.20+
- Claude Code CLI installed and authenticated
- Feishu self-built app credentials

## Configuration

All environment variables use the `DEVBOT_` prefix:

| Variable | Required | Description | Default |
|----------|----------|-------------|---------|
| `DEVBOT_APP_ID` | Yes | Feishu App ID | — |
| `DEVBOT_APP_SECRET` | Yes | Feishu App Secret | — |
| `DEVBOT_ALLOWED_USER_IDS` | Yes | Comma-separated user IDs | — |
| `DEVBOT_WORK_ROOT` | No | Root work directory | `$HOME` |
| `DEVBOT_CLAUDE_PATH` | No | Claude CLI path | `claude` |
| `DEVBOT_CLAUDE_MODEL` | No | Default model | `sonnet` |
| `DEVBOT_CLAUDE_TIMEOUT` | No | Timeout in seconds | `600` |
| `DEVBOT_STATE_FILE` | No | State file path | `~/.devbot/state.json` |

## Run

```bash
export DEVBOT_APP_ID=cli_xxx
export DEVBOT_APP_SECRET=xxx
export DEVBOT_ALLOWED_USER_IDS=user1,user2
./scripts/run.sh
```

The run script includes a keep-alive loop that automatically restarts the bot on crash.

## Commands

Send any text message directly to Claude Code. Use `/` prefix for control commands:

**Basic:**
- `/help` — Show all commands
- `/ping` — Check if bot is alive
- `/status` — Current status and Claude usage

**Directory:**
- `/root <path>` — Set root work directory
- `/cd <dir>` — Change directory (relative to root)
- `/pwd` — Show current directory
- `/ls` — List projects in root directory

**Git:**
- `/git <args>` — Run git command
- `/diff` — Show current changes
- `/commit <msg>` — Quick commit
- `/push` — Quick push
- `/undo` — Discard uncommitted changes
- `/stash [pop]` — Stash/restore changes

**Session:**
- `/new` — Start new Claude session
- `/sessions` — List session history
- `/switch <id>` — Switch to session

**Control:**
- `/kill` — Terminate current execution
- `/model <name>` — Switch Claude model
- `/yolo` — Enable unrestricted mode
- `/safe` — Restore safe mode
- `/last` — Show last Claude output
- `/summary` — Summarize last output

**System:**
- `/sh <cmd>` — Run shell command (via Claude)
- `/file <path>` — Send project file to chat

## Behavior

- Receives Feishu messages via WebSocket long-connection
- Routes text to Claude Code CLI (`claude -p`)
- Supports session continuity via `--resume`
- Group chats: only responds when @mentioned
- Private chats: responds to all messages
- Long outputs are split into multiple messages (no truncation)
- Per-chat message queue ensures sequential execution
- State persisted to `~/.devbot/state.json`
