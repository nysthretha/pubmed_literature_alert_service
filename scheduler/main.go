package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/httpapi"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/poller"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/publisher"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/pubmed"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/store"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const (
	fetchQueue = "pmid.fetch"
	httpAddr   = ":8080"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	rabbitURL := mustEnv("RABBITMQ_URL")
	pgURL := mustEnv("POSTGRES_URL")
	email := mustEnv("PUBMED_EMAIL")
	tool := getEnv("PUBMED_TOOL_NAME", "pubmed-alerts")
	apiKey := os.Getenv("PUBMED_API_KEY")

	tickSec, err := strconv.Atoi(getEnv("SCHEDULER_TICK_SECONDS", "300"))
	if err != nil || tickSec <= 0 {
		slog.Error("invalid SCHEDULER_TICK_SECONDS", "value", os.Getenv("SCHEDULER_TICK_SECONDS"))
		os.Exit(1)
	}

	db, err := openDB(pgURL)
	if err != nil {
		slog.Error("db open failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.Migrate(db, migrationsFS); err != nil {
		slog.Error("migrations failed", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")

	pub, err := publisher.New(rabbitURL, fetchQueue)
	if err != nil {
		slog.Error("rabbitmq connect failed", "err", err)
		os.Exit(1)
	}
	defer pub.Close()

	pm := pubmed.NewClient(tool, email, apiKey)
	st := store.New(db)
	p := poller.New(st, pm, pub)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	httpSrv := &http.Server{
		Addr:              httpAddr,
		Handler:           httpapi.NewRouter(db),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.Info("http server starting", "addr", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "err", err)
		}
	}()

	tick := time.Duration(tickSec) * time.Second
	slog.Info("starting poller", "tick", tick.String(), "queue", fetchQueue, "api_key_present", apiKey != "")

	if err := p.RunOnce(ctx); err != nil && ctx.Err() == nil {
		slog.Error("initial poll failed", "err", err)
	}

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down")
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = httpSrv.Shutdown(shutCtx)
			shutCancel()
			return
		case <-ticker.C:
			if err := p.RunOnce(ctx); err != nil && ctx.Err() == nil {
				slog.Error("poll failed", "err", err)
			}
		}
	}
}

func openDB(url string) (*sql.DB, error) {
	var lastErr error
	for i := 1; i <= 5; i++ {
		db, err := sql.Open("pgx", url)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err = db.PingContext(ctx)
			cancel()
			if err == nil {
				return db, nil
			}
			db.Close()
		}
		lastErr = err
		slog.Warn("db connect attempt failed", "attempt", i, "err", err)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		slog.Error("missing required env var", "name", k)
		os.Exit(1)
	}
	return v
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
