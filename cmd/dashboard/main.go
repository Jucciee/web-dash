package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"homedashboard/internal/app"
	"homedashboard/internal/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("dashboard stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}
	if cfg.LogFile == "" {
		cfg.LogFile = filepath.Join(cfg.DataDir, "logs", "dashboard.log")
	}
	closeLog, err := logging.Configure(cfg.LogLevel, cfg.LogFormat, cfg.LogFile)
	if err != nil {
		return err
	}
	defer closeLog()
	slog.Info("dashboard starting", "addr", cfg.Addr, "dataDir", cfg.DataDir, "logFile", cfg.LogFile)
	application, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer application.Close()
	server := &http.Server{Addr: cfg.Addr, Handler: application.Handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result := make(chan error, 1)
	go func() { slog.Info("dashboard listening", "addr", cfg.Addr); result <- server.ListenAndServe() }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		slog.Info("dashboard shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		if err == nil {
			slog.Info("dashboard stopped")
		}
		return err
	}
}
