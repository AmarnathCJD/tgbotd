// Command tgbotd is a from-scratch Telegram Bot API server written in Go.
// It speaks the Bot API HTTP protocol on the front and MTProto on the back,
// using gogram for the underlying MTProto transport.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/config"
	"github.com/amarnathcjd/tgbotd/internal/logx"
	"github.com/amarnathcjd/tgbotd/internal/server"
	"github.com/amarnathcjd/tgbotd/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(2)
	}
	log := logx.New(cfg.LogLevel)
	log.Info("tgbotd starting",
		"http_addr", cfg.HTTPAddr,
		"data_dir", cfg.DataDir,
		"api_id", cfg.APIID,
		"test_dc", cfg.TestDC,
	)

	store, err := storage.Open(cfg.DataDir)
	if err != nil {
		log.Error("open storage", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	mgr := botmgr.New(cfg, store, log)
	srv := server.New(cfg, mgr, log)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Prewarm all known bots concurrently in background so first HTTP hit
	// on any known token skips the 5-6s cold-start latency.
	go mgr.Prewarm(ctx)

	go func() {
		<-ctx.Done()
		log.Info("shutting down")
		mgr.Shutdown()
	}()

	if err := srv.ListenAndServe(ctx); err != nil {
		log.Error("server error", "err", err)
		os.Exit(1)
	}
	log.Info("bye")
}
