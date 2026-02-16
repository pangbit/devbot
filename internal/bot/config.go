package bot

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	AppID          string
	AppSecret      string
	AllowedUserIDs map[string]bool
	BotOpenID      string
	WorkRoot       string
	ClaudePath     string
	ClaudeModel    string
	ClaudeTimeout  int
	StateFile      string
	SkipBotSelf    bool
}

func LoadConfig() (Config, error) {
	appID := strings.TrimSpace(os.Getenv("DEVBOT_APP_ID"))
	appSecret := strings.TrimSpace(os.Getenv("DEVBOT_APP_SECRET"))
	if appID == "" || appSecret == "" {
		return Config{}, errors.New("DEVBOT_APP_ID and DEVBOT_APP_SECRET are required")
	}

	allowedRaw := strings.TrimSpace(os.Getenv("DEVBOT_ALLOWED_USER_IDS"))
	if allowedRaw == "" {
		return Config{}, errors.New("DEVBOT_ALLOWED_USER_IDS is required")
	}
	allowedUserIDs := make(map[string]bool)
	for _, id := range strings.Split(allowedRaw, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			allowedUserIDs[id] = true
		}
	}

	home, _ := os.UserHomeDir()

	workRoot := strings.TrimSpace(os.Getenv("DEVBOT_WORK_ROOT"))
	if workRoot == "" {
		workRoot = home
	}

	claudePath := strings.TrimSpace(os.Getenv("DEVBOT_CLAUDE_PATH"))
	if claudePath == "" {
		claudePath = "claude"
	}

	claudeModel := strings.TrimSpace(os.Getenv("DEVBOT_CLAUDE_MODEL"))
	if claudeModel == "" {
		claudeModel = "sonnet"
	}

	claudeTimeout := 600
	if v := strings.TrimSpace(os.Getenv("DEVBOT_CLAUDE_TIMEOUT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			claudeTimeout = n
		}
	}

	stateFile := strings.TrimSpace(os.Getenv("DEVBOT_STATE_FILE"))
	if stateFile == "" {
		stateFile = filepath.Join(home, ".devbot", "state.json")
	}

	botOpenID := strings.TrimSpace(os.Getenv("DEVBOT_BOT_OPEN_ID"))

	skipBotSelf := true
	if v := strings.TrimSpace(os.Getenv("DEVBOT_SKIP_BOT_SELF")); v == "false" || v == "0" {
		skipBotSelf = false
	}

	return Config{
		AppID:          appID,
		AppSecret:      appSecret,
		AllowedUserIDs: allowedUserIDs,
		BotOpenID:      botOpenID,
		WorkRoot:       workRoot,
		ClaudePath:     claudePath,
		ClaudeModel:    claudeModel,
		ClaudeTimeout:  claudeTimeout,
		StateFile:      stateFile,
		SkipBotSelf:    skipBotSelf,
	}, nil
}
