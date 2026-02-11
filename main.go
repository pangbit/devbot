package main

import (
    "context"
    "log"

    lark "github.com/larksuite/oapi-sdk-go/v3"

    "devbot/internal/bot"
)

func main() {
    cfg, err := bot.LoadConfig()
    if err != nil {
        log.Fatal(err)
    }

    client := lark.NewClient(cfg.AppID, cfg.AppSecret)
    sender := bot.NewLarkSender(client)
    handler := bot.NewHandler(sender, cfg.SkipBotSelf)

    if err := bot.Run(context.Background(), cfg, handler, nil); err != nil {
        log.Fatal(err)
    }
}
