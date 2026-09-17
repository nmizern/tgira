// Command tgira runs the task tracker bot.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/nmizern/tgira/internal/config"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tgira:", err)
		os.Exit(1)
	}
}

func run() error {
	path := flag.String("config", "config.yaml", "path to the config file")
	printVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *printVersion {
		fmt.Println("tgira", version)
		return nil
	}

	cfg, err := config.Load(*path)
	if err != nil {
		return err
	}

	log := newLogger(cfg.LogLevel)
	log.Info("config loaded",
		"version", version,
		"db", cfg.DBPath,
		"boards", len(cfg.Boards),
		"locale", cfg.Locale,
	)
	return nil
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
