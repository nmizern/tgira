// Command tgira runs the task tracker bot.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nmizern/tgira/internal/config"
	"github.com/nmizern/tgira/internal/store"
	"github.com/nmizern/tgira/internal/tg"
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

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := db.Migrate(ctx); err != nil {
		return err
	}

	bot, err := tg.New(cfg, db, log)
	if err != nil {
		return err
	}

	log.Info("starting", "version", version, "db", cfg.DBPath, "boards", len(cfg.Boards), "locale", cfg.Locale)
	if err := bot.Start(ctx); err != nil {
		return err
	}

	log.Info("stopped")
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
