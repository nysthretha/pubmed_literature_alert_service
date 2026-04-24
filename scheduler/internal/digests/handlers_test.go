package digests_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/digests"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/testsupport"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	db, cleanup, err := testsupport.Postgres(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres unavailable: %v\n", err)
		os.Exit(0)
	}
	if _, err := testsupport.SeedFullSchema(ctx, db); err != nil {
		fmt.Fprintf(os.Stderr, "schema setup failed: %v\n", err)
		cleanup()
		os.Exit(1)
	}
	testDB = db
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func resetTables(t *testing.T) {
	t.Helper()
	for _, stmt := range []string{
		"DELETE FROM digest_articles",
		"DELETE FROM digests",
		"DELETE FROM queries WHERE user_id != (SELECT id FROM users WHERE username = 'admin')",
		"DELETE FROM articles",
		"DELETE FROM sessions",
		"DELETE FROM users WHERE username != 'admin'",
	} {
		if _, err := testDB.Exec(stmt); err != nil {
			t.Fatalf("cleanup: %s: %v", stmt, err)
		}
	}
}

func seedDigest(t *testing.T, userID int64, articlesCount int) int64 {
	t.Helper()
	var id int64
	if err := testDB.QueryRow(`
		INSERT INTO digests (user_id, status, articles_included, manual)
		VALUES ($1, 'sent', $2, false)
		RETURNING id
	`, userID, articlesCount).Scan(&id); err != nil {
		t.Fatalf("insert digest: %v", err)
	}
	return id
}

// Regression: the digests_one_sent_per_day unique index was originally
// scoped by sent_local_date alone (migration 00004, pre-users). Migration
// 00008 made it UNIQUE(user_id, sent_local_date) so two users can each send
// one digest per day. This test fails if that migration is missing or wrong.
func TestUniqueIndex_DigestsScopedByUser(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	alice, _ := testsupport.SeedUser(ctx, testDB, "alice", "alice@test", "alicepassword", false)
	bob, _ := testsupport.SeedUser(ctx, testDB, "bob", "bob@test", "bobpassword12", false)

	localDate := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)

	// Alice sends one.
	if _, err := testDB.ExecContext(ctx, `
		INSERT INTO digests (user_id, status, sent_local_date, articles_included)
		VALUES ($1, 'sent', $2, 3)
	`, alice.ID, localDate); err != nil {
		t.Fatalf("alice first digest: %v", err)
	}

	// Bob sends one on the same date — must succeed (different user).
	if _, err := testDB.ExecContext(ctx, `
		INSERT INTO digests (user_id, status, sent_local_date, articles_included)
		VALUES ($1, 'sent', $2, 5)
	`, bob.ID, localDate); err != nil {
		t.Fatalf("bob digest on alice's date should succeed after 00008: %v", err)
	}

	// Alice tries a second sent-digest for the same date — must fail.
	_, err := testDB.ExecContext(ctx, `
		INSERT INTO digests (user_id, status, sent_local_date, articles_included)
		VALUES ($1, 'sent', $2, 7)
	`, alice.ID, localDate)
	if err == nil {
		t.Fatal("alice second sent-digest on same date should be blocked")
	}
	if !strings.Contains(err.Error(), "23505") && !strings.Contains(err.Error(), "unique") {
		t.Fatalf("expected unique-violation error, got: %v", err)
	}

	// 'failed' and 'pending' rows are NOT subject to the partial index —
	// alice can have multiple failed attempts on the same date.
	if _, err := testDB.ExecContext(ctx, `
		INSERT INTO digests (user_id, status, sent_local_date, articles_included)
		VALUES ($1, 'failed', $2, 0)
	`, alice.ID, localDate); err != nil {
		t.Fatalf("alice failed-digest row should be allowed: %v", err)
	}
}

func TestIsolation_DigestsScopedToUser(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	alice, _ := testsupport.SeedUser(ctx, testDB, "alice", "alice@test", "alicepassword", false)
	bob, _ := testsupport.SeedUser(ctx, testDB, "bob", "bob@test", "bobpassword12", false)
	aliceC, _ := testsupport.SeedSession(ctx, testDB, alice.ID)
	_, _ = testsupport.SeedSession(ctx, testDB, bob.ID)

	aliceDigest := seedDigest(t, alice.ID, 3)
	bobDigest := seedDigest(t, bob.ID, 5)

	mux := http.NewServeMux()
	digests.RegisterRoutes(mux, testDB, auth.Required(testDB))

	// Alice lists — sees only hers.
	req := httptest.NewRequest("GET", "/api/digests", nil)
	req.AddCookie(aliceC)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("alice list status=%d", w.Code)
	}
	var out struct {
		Digests []digests.DigestSummary `json:"digests"`
		Total   int                     `json:"total"`
	}
	_ = json.NewDecoder(w.Body).Decode(&out)
	if out.Total != 1 || out.Digests[0].ID != aliceDigest {
		t.Fatalf("alice got unexpected: %+v", out)
	}

	// Alice GET bob's id → 404.
	req = httptest.NewRequest("GET", "/api/digests/"+strconv.FormatInt(bobDigest, 10), nil)
	req.AddCookie(aliceC)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("alice GET bob's digest: got %d, want 404", w.Code)
	}
}
