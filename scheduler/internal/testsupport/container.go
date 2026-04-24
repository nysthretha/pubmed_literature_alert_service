// Package testsupport provides helpers for integration tests against Postgres.
// A test package calls NewContainer once in TestMain to get a shared DB, then
// wraps each test in a transaction via WithTx for isolation.
package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Postgres returns a connected *sql.DB backed by a postgres:16-alpine container.
// The caller is responsible for calling the returned cleanup function.
func Postgres(ctx context.Context) (*sql.DB, func(), error) {
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("run postgres container: %w", err)
	}

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, nil, fmt.Errorf("get dsn: %w", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, nil, fmt.Errorf("open db: %w", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = ctr.Terminate(ctx)
	}

	for i := 0; i < 10; i++ {
		if err := db.PingContext(ctx); err == nil {
			return db, cleanup, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	cleanup()
	return nil, nil, fmt.Errorf("postgres ping never succeeded")
}

// FailOnError is a small helper to avoid `if err != nil` noise in TestMain.
func FailOnError(t testing.TB, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}
