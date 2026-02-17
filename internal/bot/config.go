package bot

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
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

// yamlConfig mirrors Config for YAML unmarshalling.
type yamlConfig struct {
	AppID          string   `yaml:"app_id"`
	AppSecret      string   `yaml:"app_secret"`
	AllowedUserIDs []string `yaml:"allowed_user_ids"`
	BotOpenID      string   `yaml:"bot_open_id"`
	WorkRoot       string   `yaml:"work_root"`
	ClaudePath     string   `yaml:"claude_path"`
	ClaudeModel    string   `yaml:"claude_model"`
	ClaudeTimeout  int      `yaml:"claude_timeout"`
	StateFile      string   `yaml:"state_file"`
	SkipBotSelf    *bool    `yaml:"skip_bot_self"`
}

// LoadConfig loads configuration from environment variables only (backward compatible).
func LoadConfig() (Config, error) {
	return LoadConfigFrom("")
}

// LoadConfigFrom loads configuration with priority: flags > config file > env vars.
// If configPath is non-empty, the YAML file is read first, then env vars fill any
// remaining blanks. Callers can override individual fields after this returns
// (for command-line flag overrides).
func LoadConfigFrom(configPath string) (Config, error) {
	var yc yamlConfig

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return Config{}, err
		}
		if err := yaml.Unmarshal(data, &yc); err != nil {
			return Config{}, err
		}
	}

	// Helper: pick first non-empty string value (yaml, then env)
	pick := func(yamlVal, envKey string) string {
		if yamlVal != "" {
			return yamlVal
		}
		return strings.TrimSpace(os.Getenv(envKey))
	}

	appID := pick(yc.AppID, "DEVBOT_APP_ID")
	appSecret := pick(yc.AppSecret, "DEVBOT_APP_SECRET")
	if appID == "" || appSecret == "" {
		return Config{}, errors.New("app_id and app_secret are required (config file or DEVBOT_APP_ID / DEVBOT_APP_SECRET)")
	}

	// Allowed user IDs: yaml list, fallback to env comma-separated
	allowedUserIDs := make(map[string]bool)
	if len(yc.AllowedUserIDs) > 0 {
		for _, id := range yc.AllowedUserIDs {
			id = strings.TrimSpace(id)
			if id != "" {
				allowedUserIDs[id] = true
			}
		}
	} else {
		allowedRaw := strings.TrimSpace(os.Getenv("DEVBOT_ALLOWED_USER_IDS"))
		if allowedRaw != "" {
			for _, id := range strings.Split(allowedRaw, ",") {
				id = strings.TrimSpace(id)
				if id != "" {
					allowedUserIDs[id] = true
				}
			}
		}
	}
	if len(allowedUserIDs) == 0 {
		return Config{}, errors.New("allowed_user_ids is required (config file or DEVBOT_ALLOWED_USER_IDS)")
	}

	home, _ := os.UserHomeDir()

	workRoot := pick(yc.WorkRoot, "DEVBOT_WORK_ROOT")
	if workRoot == "" {
		workRoot = home
	}

	claudePath := pick(yc.ClaudePath, "DEVBOT_CLAUDE_PATH")
	if claudePath == "" {
		claudePath = "claude"
	}

	claudeModel := pick(yc.ClaudeModel, "DEVBOT_CLAUDE_MODEL")
	if claudeModel == "" {
		claudeModel = "sonnet"
	}

	claudeTimeout := yc.ClaudeTimeout
	if claudeTimeout <= 0 {
		if v := strings.TrimSpace(os.Getenv("DEVBOT_CLAUDE_TIMEOUT")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				claudeTimeout = n
			}
		}
	}
	if claudeTimeout <= 0 {
		claudeTimeout = 600
	}

	stateFile := pick(yc.StateFile, "DEVBOT_STATE_FILE")
	if stateFile == "" {
		stateFile = filepath.Join(home, ".devbot", "state.json")
	}

	botOpenID := pick(yc.BotOpenID, "DEVBOT_BOT_OPEN_ID")

	skipBotSelf := true
	if yc.SkipBotSelf != nil {
		skipBotSelf = *yc.SkipBotSelf
	} else if v := strings.TrimSpace(os.Getenv("DEVBOT_SKIP_BOT_SELF")); v == "false" || v == "0" {
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
