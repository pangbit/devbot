package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"devbot/internal/bot"
	"devbot/internal/version"
)

func main() {
	configPath := flag.String("c", "", "配置文件路径")
	showVersion := flag.Bool("v", false, "显示版本信息")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: devbot [flags]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n配置优先级: 命令行参数 > 配置文件 (-c) > 环境变量\n")
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	cfg, err := bot.LoadConfigFrom(*configPath)
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
	handler := bot.NewHandler(router, downloader, sender, cfg.SkipBotSelf, cfg.BotOpenID, cfg.AllowedUserIDs)

	// Signal handler only cancels the context
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("Received %s, shutting down...", sig)
		cancel()
	}()

	log.Printf("Starting devbot (%s)...", version.Version)
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
