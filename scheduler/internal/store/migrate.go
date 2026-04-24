package store

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// Migrate applies all pending goose migrations from the provided FS.
// After the M5a refactor, migrations live at the FS root (migrations/embed.go
// uses `//go:embed *.sql`), so the dir arg is ".".
func Migrate(db *sql.DB, fs embed.FS) error {
	goose.SetBaseFS(fs)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
