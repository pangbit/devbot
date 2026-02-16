package main

import (
    "context"
    "log"
    "time"

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

    store, err := bot.NewStore(cfg.StateFile)
    if err != nil {
        log.Fatal(err)
    }

    executor := bot.NewClaudeExecutor(
        cfg.ClaudePath,
        cfg.ClaudeModel,
        time.Duration(cfg.ClaudeTimeout)*time.Second,
    )

    router := bot.NewRouter(executor, store, sender, cfg.AllowedUserIDs, cfg.WorkRoot)
    handler := bot.NewHandler(router, cfg.SkipBotSelf, "")

    if err := bot.Run(context.Background(), cfg, handler, nil); err != nil {
        log.Fatal(err)
    }
}
