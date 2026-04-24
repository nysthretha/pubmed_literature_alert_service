package testsupport

import (
	"context"
	"database/sql"
	"testing"
)

// WithTx runs fn inside a transaction that is always rolled back, giving each
// test a fresh view of the database without tearing down shared containers.
//
// Note: this wraps callers in a *sql.Tx, not a *sql.DB. If the code under test
// requires *sql.DB, seed the DB directly and rely on cleanup to reset instead
// of this helper.
func WithTx(t *testing.T, db *sql.DB, fn func(*testing.T, *sql.Tx)) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	fn(t, tx)
}

// TruncateAll wipes all rows from the given tables in one statement.
// Use this between tests when the code under test needs *sql.DB (not *sql.Tx).
// Order matters: list FK children before parents.
func TruncateAll(t *testing.T, db *sql.DB, tables ...string) {
	t.Helper()
	if len(tables) == 0 {
		return
	}
	stmt := "TRUNCATE " + joinComma(tables) + " RESTART IDENTITY CASCADE"
	if _, err := db.ExecContext(context.Background(), stmt); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
