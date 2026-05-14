package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coilysiren/personal-dashboard/internal/server"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:31337", "listen address; default binds loopback only, override to Tailscale IP at deploy time")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	srv := server.New(logger, server.Config{
		ElevenLabsAPIKey:  os.Getenv("ELEVENLABS_API_KEY"),
		ElevenLabsVoiceID: os.Getenv("ELEVENLABS_VOICE_ID"),
	})

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Background pruner for the session store. Runs every 5 minutes; cheap
	// even at hundreds of sessions because the map is small.
	pruneTick := time.NewTicker(5 * time.Minute)
	defer pruneTick.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-pruneTick.C:
				if n := srv.Sessions().Prune(); n > 0 {
					logger.Info("session pruner", "evicted", n)
				}
			}
		}
	}()

	go func() {
		logger.Info("daemon listening", "addr", *addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown initiated")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "err", err)
		os.Exit(1)
	}
	logger.Info("shutdown complete")
}
