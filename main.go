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

	switch *mode {
	case "read":
		runner := api.NewReadRunner(cfg.ReadConfig, cfg.Accounts, n)
		if err := runner.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Read mode failed: %v\n", err)
			os.Exit(1)
		}
	case "write":
		runner := api.NewWriteRunner(cfg.WriteConfig, cfg.Accounts, cfg.Email, cfg.City, n)
		if err := runner.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Write mode failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s (use 'read' or 'write')\n", *mode)
		os.Exit(1)
	}
}
