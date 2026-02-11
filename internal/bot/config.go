package bot

import (
    "errors"
    "os"
    "strings"
)

type Config struct {
    AppID       string
    AppSecret   string
    SkipBotSelf bool
}

func LoadConfig() (Config, error) {
    appID := strings.TrimSpace(os.Getenv("APP_ID"))
    appSecret := strings.TrimSpace(os.Getenv("APP_SECRET"))
    if appID == "" || appSecret == "" {
        return Config{}, errors.New("APP_ID and APP_SECRET are required")
    }

    skipBotSelf := true
    if v := strings.TrimSpace(os.Getenv("SKIP_BOT_SELF")); v != "" {
        if v == "false" || v == "0" {
            skipBotSelf = false
        }
    }

    return Config{AppID: appID, AppSecret: appSecret, SkipBotSelf: skipBotSelf}, nil
}
