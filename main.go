package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
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

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
        sig := <-sigCh
        log.Printf("Received %s, shutting down...", sig)
        cancel()
    }()

    log.Println("Starting devbot...")
    if err := bot.Run(ctx, cfg, handler, nil); err != nil {
        log.Fatal(err)
    }
}
