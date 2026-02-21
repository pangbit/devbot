# DevBot — 飞书 Claude Code 控制器

通过飞书机器人远程控制 Claude Code CLI，专为移动端项目开发设计。

## 功能特性

- 飞书消息直接发送给 Claude Code，支持多轮会话
- 流式执行：长时间任务实时推送中间进度
- 命令结果以 Markdown 卡片展示，错误红色高亮
- 支持图片、文件消息（自动下载保存到工作目录）
- 飞书文档双向同步（push/pull）
- `/find` 按文件名搜索，`/grep` 按内容搜索，覆盖主流文件类型
- 群聊 @机器人 触发，私聊直接响应
- 每个聊天独立的消息队列，保证顺序执行
- 状态持久化，支持会话恢复（`/sessions` + `/switch`）
- 目录切换自动关联 Claude 会话（不同目录独立上下文）
- `/retry` 一键重试上一条请求，无需重新输入
- `/commit` 无需提交信息，Claude 自动生成
- `/branch` 分支创建/切换快捷命令
- `/info` 快速概览当前状态（目录 + git 分支 + 变更 + 运行状态）
- `/pr` 一键创建 Pull Request（Claude 自动生成描述）
- `/grep` 代码关键词搜索，覆盖主流文件类型
- `/status` 增强：实时显示 git 分支和工作区变更数量

## 环境要求

- Go 1.20+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) 已安装并完成认证
- 飞书自建应用（见下方[飞书应用配置](#飞书应用配置)）

## 快速开始

### 1. 构建

```bash
make build
```

二进制文件生成在 `dist/devbot-v<VERSION>-<OS>-<ARCH>/devbot`。

### 2. 配置

支持两种配置方式，优先级：**命令行参数 > 配置文件 > 环境变量**。

**方式一：YAML 配置文件（推荐）**

```bash
cp deploy/config.example.yaml config.yaml
# 编辑 config.yaml，填入飞书应用凭证
```

运行时通过 `-c` 指定：

```bash
./dist/devbot-*/devbot -c config.yaml
```

**方式二：环境变量**

| 变量 | 必填 | 说明 | 默认值 |
|------|------|------|--------|
| `DEVBOT_APP_ID` | 是 | 飞书 App ID | — |
| `DEVBOT_APP_SECRET` | 是 | 飞书 App Secret | — |
| `DEVBOT_ALLOWED_USER_IDS` | 是 | 允许的用户 ID（逗号分隔，支持 `open_id` 和 `user_id`） | — |
| `DEVBOT_BOT_OPEN_ID` | 否 | 机器人 Open ID（群聊 @检测） | — |
| `DEVBOT_WORK_ROOT` | 否 | 工作根目录 | `$HOME` |
| `DEVBOT_CLAUDE_PATH` | 否 | Claude CLI 路径 | `claude` |
| `DEVBOT_CLAUDE_MODEL` | 否 | 默认模型 | `sonnet` |
| `DEVBOT_CLAUDE_TIMEOUT` | 否 | 超时时间（秒） | `600` |
| `DEVBOT_STATE_FILE` | 否 | 状态文件路径 | `~/.devbot/state.json` |
| `DEVBOT_SKIP_BOT_SELF` | 否 | 忽略机器人自身消息 | `true` |

### 3. 运行

```bash
# 使用配置文件
./dist/devbot-*/devbot -c config.yaml

# 使用环境变量
export DEVBOT_APP_ID=cli_xxx
export DEVBOT_APP_SECRET=xxx
export DEVBOT_ALLOWED_USER_IDS=ou_xxx
./dist/devbot-*/devbot
```

## 飞书应用配置

### 1. 创建自建应用

1. 进入[飞书开放平台](https://open.feishu.cn) → 开发者后台
2. 点击「创建企业自建应用」
3. 填写应用名称和描述
4. 在「凭证与基础信息」页面记录 **App ID** 和 **App Secret**

### 2. 开通权限

在应用后台 → **权限管理** 中申请以下权限：

| 权限标识 | 说明 |
|----------|------|
| `docx:document` | 读写飞书文档（/doc 命令） |
| `im:message` | 发送消息和卡片 |
| `im:message.p2p_msg:readonly` | 接收私聊消息 |
| `im:message.group_at_msg:readonly` | 接收群聊 @ 消息 |
| `contact:user.employee_id:readonly` | 通过 user_id 识别用户 |

### 3. 配置事件订阅

在应用后台 → **事件与回调** → **添加事件**：

- 添加事件：`im.message.receive_v1`（接收消息）
- 订阅方式选择：**使用长连接接收事件（WebSocket）**

> 使用长连接无需公网地址，适合本地或内网部署。

### 4. 启用机器人

在应用后台 → **应用功能** → **机器人** → 开启机器人功能。

### 5. 获取 Bot Open ID（可选，用于群聊 @ 检测）

将机器人加入任意群聊后，发送一条消息，在日志中找到 `bot_open_id`，或：

1. 进入应用后台 → **机器人** 页面，复制机器人的 Open ID
2. 填入配置项 `bot_open_id` 或环境变量 `DEVBOT_BOT_OPEN_ID`

> 不配置此项时，群聊消息会被忽略，仅支持私聊。

### 6. 获取用户 ID

在飞书管理后台 → **成员** 中找到允许使用的用户，复制其 **Open ID**（`ou_` 前缀）或 **User ID**，填入 `allowed_user_ids`。

### 7. 发布应用

完成配置后，在应用后台点击「创建版本」并提交审核（企业内部应用通常即时生效）。

## 部署

### 打包

```bash
make package
```

生成 `dist/devbot-v<VERSION>-<OS>-<ARCH>.tar.gz`，包含二进制、配置示例、systemd service 和安装脚本。

### 安装到服务器

```bash
# 1. 上传到服务器
scp dist/devbot-*.tar.gz user@server:/tmp/

# 2. 在服务器上解压并安装（需要 root 权限）
ssh user@server
tar -xzf /tmp/devbot-*.tar.gz -C /tmp/
sudo bash /tmp/devbot-*/install.sh

# 3. 编辑配置文件
sudo vim /opt/devbot/config.yaml

# 4. 启动服务
sudo systemctl start devbot
sudo systemctl enable devbot   # 开机自启
```

### 管理服务

```bash
systemctl status devbot        # 查看状态
systemctl restart devbot       # 重启
journalctl -u devbot -f        # 实时日志
tail -f /opt/devbot/devbot.log # 文件日志
```

## 命令参考

直接发送文本消息即可与 Claude Code 对话。使用 `/` 前缀发送控制命令：

**基础：**
- `/help` — 显示所有命令
- `/ping` — 检查机器人在线状态和运行时长
- `/info` — 快速概览（目录、分支、工作区变更、模型、运行状态）
- `/status` — 详细状态（含 git 分支、变更信息、执行统计）

**目录：**
- `/root [path]` — 查看/设置工作根目录（必须为绝对路径）
- `/cd <dir>` — 切换目录（相对于根目录，失败时显示可用目录）
- `/pwd` — 显示当前目录
- `/ls` — 列出根目录下的项目

**Git：**
- `/git <args>` — 执行任意 git 命令
- `/diff` — 查看当前变更
- `/log [n]` — 查看提交历史（默认最近 20 条）
- `/branch [name]` — 查看分支列表，或创建/切换分支
- `/commit [msg]` — 提交变更（不填消息则 Claude 自动生成）
- `/push` — 推送到远程
- `/pr [title]` — 创建 Pull Request（使用 gh CLI）
- `/undo` — 撤销所有未提交的更改
- `/stash [pop]` — 暂存/恢复更改

**会话：**
- `/new` — 开始新的 Claude 会话（旧会话保存到历史）
- `/sessions` — 列出会话历史（含序号，可用 `/switch 0` 恢复）
- `/switch <id|序号>` — 切换到指定会话

**控制：**
- `/kill` / `/cancel` — 终止正在执行的任务
- `/retry` — 重试上一条发给 Claude 的消息
- `/model [name]` — 查看/切换模型（haiku/sonnet/opus）
- `/yolo` — 开启无限制模式（Claude 可执行所有操作，显示风险警告）
- `/safe` — 恢复安全模式
- `/last` — 显示上次 Claude 输出
- `/summary` — 让 Claude 总结上次输出
- `/compact` — 压缩当前对话上下文（节省 token，延长会话生命周期）

**搜索与文件：**
- `/grep <pattern>` — 在代码中搜索关键词（支持多种文件类型）
- `/find <name>` — 按文件名查找文件（支持通配符，如 `*.go`）
- `/test [pattern]` — 运行项目测试（自动识别 Go/Node/Python/Rust）
- `/recent [n]` — 列出最近修改的 n 个文件（默认 10 个）
- `/debug` — 分析上次输出中的错误并给出修复建议
- `/exec <cmd>` — 直接执行 Shell 命令（即时返回，无需 Claude，适合 `ls`、`make`、`go test` 等）
- `/sh <cmd>` — 通过 Claude 执行 Shell 命令（带 AI 解释）
- `/file <path>` — 发送项目文件内容到聊天

**飞书文档同步：**
- `/doc push <path>` — 将 Markdown 文件推送到飞书文档
- `/doc pull <path>` — 将飞书文档内容拉取到本地文件
- `/doc bind <path> <url|id>` — 绑定本地文件到飞书文档
- `/doc unbind <path>` — 解除绑定
- `/doc list` — 列出所有绑定关系

## 消息展示

- **命令结果**：Markdown 卡片格式，支持加粗、代码块、链接等
- **流式进度**：长时间任务（>5秒）每 10 秒推送一次中间结果卡片
- **执行完成**：纯文本 `✓ 完成（耗时 Xs）`
- **错误**：红色卡片显示错误信息和耗时
- **权限确认**：紫色卡片，提示用 `/yolo` 跳过确认
- **排队**：蓝色卡片显示队列位置，满队时提示稍后重试

## 架构

- 通过 WebSocket 长连接接收飞书消息
- 使用 Claude Code CLI（`claude -p --output-format stream-json`）流式执行
- 支持 `--resume` 会话续接
- 长输出自动分片发送（不截断）
- 状态持久化到 `~/.devbot/state.json`
- 飞书文档分享卡片自动识别用于绑定
- SIGINT/SIGTERM 优雅关闭

## 常见问题

### 机器人收不到消息

1. 确认已在飞书开放平台订阅 `im.message.receive_v1` 事件
2. 确认事件订阅方式选择了**长连接（WebSocket）**，而非 HTTP 回调
3. 确认应用已发布（或使用测试版本）
4. 检查日志是否有连接错误

### 群聊 @ 不响应

1. 确认机器人已被加入该群聊
2. 确认配置了 `bot_open_id`
3. 确认发消息的用户在 `allowed_user_ids` 列表中
4. 确认开通了 `im:message.group_at_msg:readonly` 权限

### Claude CLI 找不到

```bash
# 确认 claude 命令可用
which claude
claude --version

# 若不在 PATH 中，在配置文件中指定完整路径
claude_path: "/usr/local/bin/claude"
```

### 执行超时

默认超时 600 秒（10 分钟）。复杂任务可适当调大：

```yaml
claude_timeout: 1800   # 30 分钟
```

或环境变量：

```bash
export DEVBOT_CLAUDE_TIMEOUT=1800
```
