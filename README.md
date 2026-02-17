# DevBot — 飞书 Claude Code 控制器

通过飞书机器人远程控制 Claude Code CLI，专为移动端项目开发设计。

## 功能特性

- 飞书消息直接发送给 Claude Code，支持多轮会话
- 流式执行：长时间任务实时推送中间进度
- 命令结果以 Markdown 卡片展示
- 支持图片、文件消息（自动下载保存到工作目录）
- 飞书文档双向同步（push/pull）
- 群聊 @机器人 触发，私聊直接响应
- 每个聊天独立的消息队列，保证顺序执行
- 状态持久化，支持会话恢复

## 环境要求

- Go 1.20+
- Claude Code CLI 已安装并认证
- 飞书自建应用凭证

## 配置

所有环境变量使用 `DEVBOT_` 前缀：

| 变量 | 必填 | 说明 | 默认值 |
|------|------|------|--------|
| `DEVBOT_APP_ID` | 是 | 飞书 App ID | — |
| `DEVBOT_APP_SECRET` | 是 | 飞书 App Secret | — |
| `DEVBOT_ALLOWED_USER_IDS` | 是 | 允许的用户 ID（逗号分隔，支持 `open_id` 和 `user_id`） | — |
| `DEVBOT_WORK_ROOT` | 否 | 工作根目录 | `$HOME` |
| `DEVBOT_CLAUDE_PATH` | 否 | Claude CLI 路径 | `claude` |
| `DEVBOT_CLAUDE_MODEL` | 否 | 默认模型 | `sonnet` |
| `DEVBOT_CLAUDE_TIMEOUT` | 否 | 超时时间（秒） | `600` |
| `DEVBOT_STATE_FILE` | 否 | 状态文件路径 | `~/.devbot/state.json` |
| `DEVBOT_BOT_OPEN_ID` | 否 | 机器人 Open ID（群聊 @检测） | — |
| `DEVBOT_SKIP_BOT_SELF` | 否 | 忽略机器人自身消息 | `true` |

## 运行

```bash
export DEVBOT_APP_ID=cli_xxx
export DEVBOT_APP_SECRET=xxx
export DEVBOT_ALLOWED_USER_IDS=user1,user2
./scripts/run.sh
```

运行脚本内置保活机制，崩溃后自动重启。

## 命令

直接发送文本消息即可与 Claude Code 对话。使用 `/` 前缀发送控制命令：

**基础：**
- `/help` — 显示所有命令
- `/ping` — 检查机器人状态
- `/status` — 当前状态和执行信息

**目录：**
- `/root <path>` — 设置工作根目录
- `/cd <dir>` — 切换目录（相对于根目录）
- `/pwd` — 显示当前目录
- `/ls` — 列出根目录下的项目

**Git：**
- `/git <args>` — 执行 git 命令
- `/diff` — 查看当前更改
- `/commit <msg>` — 快速提交
- `/push` — 快速推送
- `/undo` — 丢弃未提交的更改
- `/stash [pop]` — 暂存/恢复更改

**会话：**
- `/new` — 开始新的 Claude 会话
- `/sessions` — 列出会话历史
- `/switch <id>` — 切换到指定会话

**控制：**
- `/kill` — 终止当前执行
- `/model <name>` — 切换 Claude 模型
- `/yolo` — 启用无限制模式
- `/safe` — 恢复安全模式
- `/last` — 显示上次 Claude 输出
- `/summary` — 总结上次输出

**系统：**
- `/sh <cmd>` — 运行 Shell 命令（通过 Claude）
- `/file <path>` — 发送项目文件到聊天

**文档：**
- `/doc push <path>` — 推送 Markdown 文件到飞书文档
- `/doc pull <path>` — 拉取飞书文档内容到本地文件
- `/doc bind <path> <url|id>` — 绑定本地文件到飞书文档
- `/doc unbind <path>` — 解除绑定
- `/doc list` — 列出所有绑定关系

## 消息展示

- **命令结果**：Markdown 卡片格式，支持加粗、代码块、链接等
- **执行开始**：纯文本 `Executing...`
- **流式进度**：长时间任务（>5秒）每 10 秒推送一次中间结果卡片
- **执行完成**：纯文本 `Done (耗时)`
- **错误**：红色卡片显示错误信息和耗时
- **权限确认**：紫色卡片显示 Claude 的确认请求

## 架构

- 通过 WebSocket 长连接接收飞书消息
- 使用 Claude Code CLI（`claude -p --output-format stream-json`）流式执行
- 支持 `--resume` 会话续接
- 长输出自动分片发送（不截断）
- 状态持久化到 `~/.devbot/state.json`
- 飞书文档分享卡片自动识别用于绑定
- SIGINT/SIGTERM 优雅关闭
