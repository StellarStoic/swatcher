package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/s-watcher/s-watcher/internal/app"
	"github.com/s-watcher/s-watcher/internal/notify"
	"github.com/s-watcher/s-watcher/internal/webauth"
)

const testMessage = "You receive this message because you enabled Notifications in s-watcher. Consider this a test message."

func main() {
	if len(os.Args) == 2 && os.Args[1] == "set-web-password" {
		password, err := io.ReadAll(io.LimitReader(os.Stdin, 2049))
		if err != nil {
			log.Fatal("read password")
		}
		if len(password) > 2048 {
			log.Fatal("password is too long")
		}
		path := filepath.Join(env("SWATCHER_DATA", "/data"), "auth.json")
		if err := webauth.SetPassword(path, string(password)); err != nil {
			log.Fatal(err)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "test-notification" {
		if err := testNotification(os.Args[2]); err != nil {
			log.Fatal(err)
		}
		return
	}
	listen := env("SWATCHER_LISTEN", ":8080")
	dataDir := env("SWATCHER_DATA", "/data")
	electrum := env("ELECTRUM_ADDR", "127.0.0.1:50001")
	interval, err := time.ParseDuration(env("SWATCHER_POLL_INTERVAL", "30s"))
	if err != nil || interval < time.Second {
		log.Fatal("SWATCHER_POLL_INTERVAL must be at least one second")
	}

	server, err := app.New(dataDir, electrum, interval)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("s-watcher listening on %s; electrum=%s", listen, electrum)
	log.Fatal(server.Run(listen))
}

func testNotification(channel string) error {
	sender := notify.Sender{Path: filepath.Join(env("SWATCHER_DATA", "/data"), "notifications.json")}
	config, err := sender.EnsureIdentity()
	if err != nil {
		return fmt.Errorf("load notification configuration: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	switch channel {
	case "telegram":
		if !config.TelegramEnabled {
			return fmt.Errorf("Telegram notifications are disabled")
		}
		if err := sender.Telegram(ctx, config, testMessage); err != nil {
			return fmt.Errorf("send Telegram test: %w", err)
		}
	case "nostr":
		if !config.NostrEnabled {
			return fmt.Errorf("Nostr notifications are disabled")
		}
		if err := sender.Nostr(ctx, config, testMessage); err != nil {
			return fmt.Errorf("send Nostr test: %w", err)
		}
	default:
		return fmt.Errorf("unknown notification channel %q", channel)
	}
	return nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
