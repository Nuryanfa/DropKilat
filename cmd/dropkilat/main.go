package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nuryanfa/DropKilat/internal/platform/config"
	"github.com/Nuryanfa/DropKilat/internal/platform/db"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if len(os.Args) > 1 {
		if err := migrateCommand(context.Background(), os.Args[1:], os.Getenv, os.Stdout); err != nil {
			logger.Error("migration command failed")
			os.Exit(1)
		}
		return
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database configuration failed")
		os.Exit(1)
	}
	defer pool.Close()
	mux := http.NewServeMux()
	mux.Handle("/healthz", healthHandler())
	mux.Handle("GET /readyz", readyHandler(func(ctx context.Context) error { return db.Ready(ctx, pool) }))
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		logger.Error("HTTP listener failed")
		os.Exit(1)
	}
	if err := serve(ctx, listener, mux); err != nil {
		logger.Error("HTTP server failed")
		os.Exit(1)
	}
}

func healthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			slog.Error("health response failed")
		}
	})
	return mux
}

// serve drains requests before returning. Workers can share ctx without cancelling request contexts.
func serve(ctx context.Context, listener net.Listener, handler http.Handler) error {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return errors.Join(err, server.Close())
		}
		err := <-done
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func migrateCommand(ctx context.Context, args []string, getenv func(string) string, output io.Writer) error {
	if len(args) != 2 || args[0] != "migrate" || (args[1] != "up" && args[1] != "down" && args[1] != "version") {
		return errors.New("usage: dropkilat migrate up|down|version")
	}
	url := getenv("DK_MIGRATE_DATABASE_URL")
	if url == "" {
		return errors.New("DK_MIGRATE_DATABASE_URL is required")
	}
	if args[1] == "version" {
		version, dirty, err := db.Version(ctx, url)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "%d dirty=%t\n", version, dirty)
		return err
	}
	return db.Migrate(ctx, url, args[1])
}

func readyHandler(check func(context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status, body := http.StatusOK, `{"status":"ok"}`
		if err := check(ctx); err != nil {
			status, body = http.StatusServiceUnavailable, `{"status":"unavailable"}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if _, err := w.Write([]byte(body)); err != nil {
			slog.Error("readiness response failed")
		}
	})
}
