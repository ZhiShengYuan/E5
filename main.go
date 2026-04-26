package main

import (
	"flag"
	"fmt"
	"os"

	"e5autocaller/api"
	"e5autocaller/config"
	"e5autocaller/notifier"
)

func main() {
	var (
		configPath = flag.String("config", "config.json", "Path to configuration file")
		mode       = flag.String("mode", "read", "Execution mode: read or write")
	)
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	var n notifier.Notifier = notifier.NewTelegramNotifier(cfg.Telegram.BotToken, cfg.Telegram.ChatID)

	var runErr error
	switch *mode {
	case "read":
		runner := api.NewReadRunner(cfg, n)
		runErr = runner.Run()
	case "write":
		runner := api.NewWriteRunner(cfg, n)
		runErr = runner.Run()
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s (use 'read' or 'write')\n", *mode)
		os.Exit(1)
	}

	// 无论运行成功与否，持久化可能已更新的 refresh_token
	if saveErr := config.Save(*configPath, cfg); saveErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", saveErr)
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "%s mode failed: %v\n", *mode, runErr)
		os.Exit(1)
	}
}
