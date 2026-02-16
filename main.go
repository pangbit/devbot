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

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    docSyncer := bot.NewDocSyncer(client)
    router := bot.NewRouter(ctx, executor, store, sender, cfg.AllowedUserIDs, cfg.WorkRoot, docSyncer)
    queue := bot.NewMessageQueue()
    router.SetQueue(queue)
    downloader := bot.NewLarkDownloader(client)
    handler := bot.NewHandler(router, downloader, sender, cfg.SkipBotSelf, cfg.BotOpenID)

    // Signal handler only cancels the context
    go func() {
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
        sig := <-sigCh
        log.Printf("Received %s, shutting down...", sig)
        cancel()
    }()

    log.Println("Starting devbot...")
    if err := bot.Run(ctx, cfg, handler, nil); err != nil {
        // Only fatal if not caused by context cancellation
        if ctx.Err() == nil {
            log.Fatal(err)
        }
        log.Printf("bot.Run stopped: %v", err)
    }

    // Cleanup runs after bot.Run returns, so main() won't exit prematurely
    if executor.IsRunning() {
        log.Println("Waiting for current execution to finish...")
        if executor.WaitIdle(30 * time.Second) {
            log.Println("Execution finished.")
        } else {
            log.Println("Timed out waiting, forcing shutdown.")
            executor.Kill()
        }
    }

    queue.Shutdown()
    log.Println("Shutdown complete.")
}
