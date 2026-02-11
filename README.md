# Feishu Long-Connection Bot Demo

## Requirements

- Go 1.20+
- Feishu self-built app credentials (APP_ID / APP_SECRET)

## Run

```bash
export APP_ID=cli_xxx
export APP_SECRET=xxx
export SKIP_BOT_SELF=true

go run .
```

## Behavior

- Receives `im.message.receive_v1` text events via WS
- Echoes text back to the same chat
