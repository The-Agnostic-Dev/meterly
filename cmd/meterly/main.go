// Command meterly runs the Meterly web app with seeded demo data.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/palmgrove-estates/meterly/internal/meters"
	"github.com/palmgrove-estates/meterly/internal/seed"
	"github.com/palmgrove-estates/meterly/internal/store"
	"github.com/palmgrove-estates/meterly/internal/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel()}))

	st := store.New()
	seed.Load(st)

	vendDelay := 400 * time.Millisecond
	if v := os.Getenv("METERLY_VEND_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			vendDelay = d
		}
	}

	srv, err := web.New(web.Deps{Store: st, Vendor: meters.NewDemoVendor(vendDelay), Log: log})
	if err != nil {
		log.Error("start", "err", err)
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "5120"
	}
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		log.Info("Meterly listening", "addr", "http://localhost:"+port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen", "err", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdown)
}

func logLevel() slog.Level {
	if os.Getenv("METERLY_DEBUG") != "" {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}
