package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/httpapi"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/poller"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/publisher"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/pubmed"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/store"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/migrations"
)

// httpAddrFrom returns the bind address for the HTTP server. Railway injects
// $PORT per service; in dev / compose we use the pinned 8080. Binds to all
// interfaces so it's reachable through Docker's port mapping and Railway's
// proxy; restriction to localhost in dev happens at the compose layer.
func httpAddrFrom(env func(string) string) string {
	if p := env("PORT"); p != "" {
		return ":" + p
	}
	return ":8080"
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if dispatchCLI() {
		return
	}

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

	if err := store.Migrate(db, migrations.FS); err != nil {
		slog.Error("migrations failed", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")

	pub, err := publisher.New(rabbitURL)
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

	cookieSecure := getEnv("AUTH_COOKIE_SECURE", "true") != "false"
	if !cookieSecure {
		slog.Warn("auth.cookie.secure_disabled",
			"note", "Secure cookie flag disabled — do not use in production",
			"env", "AUTH_COOKIE_SECURE=false",
		)
	}
	authCfg := auth.Config{
		DB:           db,
		RateLimiter:  auth.NewLoginRateLimiter(),
		CookieSecure: cookieSecure,
	}

	httpAddr := httpAddrFrom(os.Getenv)
	httpSrv := &http.Server{
		Addr:              httpAddr,
		Handler:           httpapi.NewRouter(db, pub, authCfg, webAssets, "web/dist"),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.Info("http server starting", "addr", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "err", err)
		}
	}()

	tick := time.Duration(tickSec) * time.Second
	slog.Info("starting poller", "tick", tick.String(), "queue", publisher.FetchQueue, "api_key_present", apiKey != "")

	// Session sweep: run once at startup, then hourly in the background.
	go runSessionSweep(ctx, db)

	// Optional external liveness ping. If HEALTHCHECK_URL is unset (dev or
	// staging), the goroutine logs once and exits — no pings, no noise.
	go runHealthcheckPing(ctx, os.Getenv("HEALTHCHECK_URL"))

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

// runHealthcheckPing sends a GET to HEALTHCHECK_URL every 5 minutes so an
// external monitor (Healthchecks.io in production) can alert on silence. The
// pings are fire-and-forget — a failed ping is logged but doesn't crash the
// scheduler, because the job of the ping is to notice ABSENCE, not presence
// of individual failures.
func runHealthcheckPing(ctx context.Context, url string) {
	if url == "" {
		slog.Warn("healthcheck.disabled", "note", "HEALTHCHECK_URL not set — skipping external liveness pings")
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	ping := func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			slog.Warn("healthcheck.request_build_failed", "err", err)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			slog.Warn("healthcheck.ping_failed", "err", err)
			return
		}
		resp.Body.Close()
	}

	ping() // one immediately so the first alert window doesn't need 5 min
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ping()
		}
	}
}

func runSessionSweep(ctx context.Context, db *sql.DB) {
	sweep := func() {
		n, err := auth.SweepExpiredSessions(ctx, db)
		if err != nil {
			slog.Error("auth.session.sweep_failed", "err", err)
			return
		}
		slog.Info("auth.session.sweep", "deleted", n)
	}
	sweep() // one at startup to catch anything from downtime

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweep()
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
