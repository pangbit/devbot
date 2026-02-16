package bot

import (
	"os"
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
