package auth_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/testsupport"
)

// testDB is the package-wide connection to the test Postgres container.
// nil if the container failed to come up (tests that need DB will t.Skip).
var testDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	db, cleanup, err := testsupport.Postgres(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres container unavailable (is Docker running?): %v\n", err)
		os.Exit(0)
	}
	if err := testsupport.MigrateUpTo(db, 5); err != nil {
		fmt.Fprintf(os.Stderr, "migrate to 00005 failed: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	testDB = db
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// resetDB wipes users + sessions between tests.
func resetDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("no test DB available")
	}
	testsupport.TruncateAll(t, testDB, "sessions", "users")
}
