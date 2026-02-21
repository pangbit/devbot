package bot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigMissing(t *testing.T) {
	t.Setenv("DEVBOT_APP_ID", "")
	t.Setenv("DEVBOT_APP_SECRET", "")
	t.Setenv("DEVBOT_ALLOWED_USER_IDS", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected error when required env vars missing")
	}
}

func TestLoadConfigAllowedUserIDsMissing(t *testing.T) {
	t.Setenv("DEVBOT_APP_ID", "cli_test")
	t.Setenv("DEVBOT_APP_SECRET", "secret")
	t.Setenv("DEVBOT_ALLOWED_USER_IDS", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected error when DEVBOT_ALLOWED_USER_IDS missing")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("DEVBOT_APP_ID", "cli_test")
	t.Setenv("DEVBOT_APP_SECRET", "secret")
	t.Setenv("DEVBOT_ALLOWED_USER_IDS", "user1,user2")
	t.Setenv("DEVBOT_CLAUDE_TIMEOUT", "")
	t.Setenv("DEVBOT_CLAUDE_PATH", "")
	t.Setenv("DEVBOT_CLAUDE_MODEL", "")
	t.Setenv("DEVBOT_WORK_ROOT", "")
	t.Setenv("DEVBOT_STATE_FILE", "")
	t.Setenv("DEVBOT_SKIP_BOT_SELF", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppID != "cli_test" || cfg.AppSecret != "secret" {
		t.Fatalf("AppID/AppSecret mismatch")
	}
	if len(cfg.AllowedUserIDs) != 2 || cfg.AllowedUserIDs["user1"] != true || cfg.AllowedUserIDs["user2"] != true {
		t.Fatalf("AllowedUserIDs mismatch: %v", cfg.AllowedUserIDs)
	}
	if cfg.ClaudePath != "claude" {
		t.Fatalf("expected default ClaudePath 'claude', got %q", cfg.ClaudePath)
	}
	if cfg.ClaudeModel != "sonnet" {
		t.Fatalf("expected default ClaudeModel 'sonnet', got %q", cfg.ClaudeModel)
	}
	if cfg.ClaudeTimeout != 600 {
		t.Fatalf("expected default ClaudeTimeout 600, got %d", cfg.ClaudeTimeout)
	}
	home, _ := os.UserHomeDir()
	if cfg.WorkRoot != home {
		t.Fatalf("expected default WorkRoot %q, got %q", home, cfg.WorkRoot)
	}
	if cfg.StateFile != home+"/.devbot/state.json" {
		t.Fatalf("expected default StateFile, got %q", cfg.StateFile)
	}
	if !cfg.SkipBotSelf {
		t.Fatalf("expected default SkipBotSelf true")
	}
}

func TestLoadConfigCustomValues(t *testing.T) {
	t.Setenv("DEVBOT_APP_ID", "cli_test")
	t.Setenv("DEVBOT_APP_SECRET", "secret")
	t.Setenv("DEVBOT_ALLOWED_USER_IDS", "admin")
	t.Setenv("DEVBOT_WORK_ROOT", "/tmp/work")
	t.Setenv("DEVBOT_CLAUDE_PATH", "/usr/local/bin/claude")
	t.Setenv("DEVBOT_CLAUDE_MODEL", "opus")
	t.Setenv("DEVBOT_CLAUDE_TIMEOUT", "300")
	t.Setenv("DEVBOT_STATE_FILE", "/tmp/state.json")
	t.Setenv("DEVBOT_SKIP_BOT_SELF", "false")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WorkRoot != "/tmp/work" {
		t.Fatalf("WorkRoot mismatch: %q", cfg.WorkRoot)
	}
	if cfg.ClaudePath != "/usr/local/bin/claude" {
		t.Fatalf("ClaudePath mismatch")
	}
	if cfg.ClaudeModel != "opus" {
		t.Fatalf("ClaudeModel mismatch")
	}
	if cfg.ClaudeTimeout != 300 {
		t.Fatalf("ClaudeTimeout mismatch")
	}
	if cfg.StateFile != "/tmp/state.json" {
		t.Fatalf("StateFile mismatch")
	}
	if cfg.SkipBotSelf {
		t.Fatalf("SkipBotSelf should be false")
	}
}

func TestLoadConfigFromYAML(t *testing.T) {
	// Clear env vars so YAML is the sole source
	t.Setenv("DEVBOT_APP_ID", "")
	t.Setenv("DEVBOT_APP_SECRET", "")
	t.Setenv("DEVBOT_ALLOWED_USER_IDS", "")

	yamlContent := `
app_id: "yaml_app"
app_secret: "yaml_secret"
allowed_user_ids:
  - "user_a"
  - "user_b"
work_root: "/yaml/work"
claude_path: "/yaml/claude"
claude_model: "haiku"
claude_timeout: 120
state_file: "/yaml/state.json"
bot_open_id: "bot_yaml"
skip_bot_self: false
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(tmpFile, []byte(yamlContent), 0644)

	cfg, err := LoadConfigFrom(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppID != "yaml_app" {
		t.Fatalf("AppID: got %q", cfg.AppID)
	}
	if cfg.AppSecret != "yaml_secret" {
		t.Fatalf("AppSecret mismatch")
	}
	if len(cfg.AllowedUserIDs) != 2 || !cfg.AllowedUserIDs["user_a"] || !cfg.AllowedUserIDs["user_b"] {
		t.Fatalf("AllowedUserIDs mismatch: %v", cfg.AllowedUserIDs)
	}
	if cfg.WorkRoot != "/yaml/work" {
		t.Fatalf("WorkRoot: got %q", cfg.WorkRoot)
	}
	if cfg.ClaudePath != "/yaml/claude" {
		t.Fatalf("ClaudePath: got %q", cfg.ClaudePath)
	}
	if cfg.ClaudeModel != "haiku" {
		t.Fatalf("ClaudeModel: got %q", cfg.ClaudeModel)
	}
	if cfg.ClaudeTimeout != 120 {
		t.Fatalf("ClaudeTimeout: got %d", cfg.ClaudeTimeout)
	}
	if cfg.StateFile != "/yaml/state.json" {
		t.Fatalf("StateFile: got %q", cfg.StateFile)
	}
	if cfg.BotOpenID != "bot_yaml" {
		t.Fatalf("BotOpenID: got %q", cfg.BotOpenID)
	}
	if cfg.SkipBotSelf {
		t.Fatalf("SkipBotSelf should be false")
	}
}

func TestLoadConfigYAMLOverridesEnv(t *testing.T) {
	// Set env vars that should be overridden by YAML
	t.Setenv("DEVBOT_APP_ID", "env_app")
	t.Setenv("DEVBOT_APP_SECRET", "env_secret")
	t.Setenv("DEVBOT_ALLOWED_USER_IDS", "env_user")
	t.Setenv("DEVBOT_CLAUDE_MODEL", "env_model")

	yamlContent := `
app_id: "yaml_app"
app_secret: "yaml_secret"
allowed_user_ids:
  - "yaml_user"
claude_model: "yaml_model"
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(tmpFile, []byte(yamlContent), 0644)

	cfg, err := LoadConfigFrom(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// YAML values should win
	if cfg.AppID != "yaml_app" {
		t.Fatalf("expected YAML app_id, got %q", cfg.AppID)
	}
	if cfg.ClaudeModel != "yaml_model" {
		t.Fatalf("expected YAML model, got %q", cfg.ClaudeModel)
	}
	if !cfg.AllowedUserIDs["yaml_user"] || cfg.AllowedUserIDs["env_user"] {
		t.Fatalf("expected YAML user IDs, got %v", cfg.AllowedUserIDs)
	}
}

func TestLoadConfigYAMLPartialWithEnvFallback(t *testing.T) {
	// YAML only has app_id/secret, rest from env
	t.Setenv("DEVBOT_APP_ID", "")
	t.Setenv("DEVBOT_APP_SECRET", "")
	t.Setenv("DEVBOT_ALLOWED_USER_IDS", "env_user")
	t.Setenv("DEVBOT_CLAUDE_MODEL", "env_model")

	yamlContent := `
app_id: "yaml_app"
app_secret: "yaml_secret"
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(tmpFile, []byte(yamlContent), 0644)

	cfg, err := LoadConfigFrom(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppID != "yaml_app" {
		t.Fatalf("AppID: got %q", cfg.AppID)
	}
	// Env fallback for allowed_user_ids and claude_model
	if !cfg.AllowedUserIDs["env_user"] {
		t.Fatalf("expected env user IDs fallback, got %v", cfg.AllowedUserIDs)
	}
	if cfg.ClaudeModel != "env_model" {
		t.Fatalf("expected env model fallback, got %q", cfg.ClaudeModel)
	}
}

func TestLoadConfigFromMissingFile(t *testing.T) {
	_, err := LoadConfigFrom("/nonexistent/config.yaml")
	if err == nil {
		t.Fatalf("expected error for missing config file")
	}
}
