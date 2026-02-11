package bot

import "testing"

func TestLoadConfigMissing(t *testing.T) {
    t.Setenv("APP_ID", "")
    t.Setenv("APP_SECRET", "")

    _, err := LoadConfig()
    if err == nil {
        t.Fatalf("expected error when APP_ID or APP_SECRET missing")
    }
}
