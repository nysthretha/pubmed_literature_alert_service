package testsupport

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/migrations"
)

// MigrateUpTo applies migrations up to and including the given version.
// Pass 0 to apply ALL migrations.
func MigrateUpTo(db *sql.DB, version int64) error {
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if version == 0 {
		return goose.Up(db, ".")
	}
	return goose.UpTo(db, ".", version)
}
