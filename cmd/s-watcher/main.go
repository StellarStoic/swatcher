package main

import (
	"log"
	"os"
	"time"

	"github.com/s-watcher/s-watcher/internal/app"
)

func main() {
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

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
