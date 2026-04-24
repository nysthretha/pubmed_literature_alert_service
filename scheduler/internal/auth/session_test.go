package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
)

func seedUser(t *testing.T) int64 {
	t.Helper()
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := auth.CreateUser(context.Background(), testDB, "alice", "alice@example.com", hash, false)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u.ID
}

func TestCreateSession(t *testing.T) {
	resetDB(t)
	uid := seedUser(t)

	sess, err := auth.CreateSession(context.Background(), testDB, uid, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("session id empty")
	}
	if sess.UserID != uid {
		t.Fatalf("user_id mismatch: got %d want %d", sess.UserID, uid)
	}
	if !sess.ExpiresAt.After(sess.CreatedAt) {
		t.Fatal("expires_at must be after created_at")
	}
}

func TestSweepDeletesExpiredSessions(t *testing.T) {
	resetDB(t)
	uid := seedUser(t)

	// Two sessions: one valid, one expired (backdated).
	sess, err := auth.CreateSession(context.Background(), testDB, uid, "", "")
	if err != nil {
		t.Fatalf("create valid: %v", err)
	}
	// Manually age one session past absolute TTL.
	_, err = testDB.ExecContext(context.Background(),
		"INSERT INTO sessions (id, user_id, created_at, expires_at, last_seen_at) VALUES ($1,$2,$3,$4,$5)",
		"expired-session-id", uid,
		time.Now().Add(-60*24*time.Hour), time.Now().Add(-30*24*time.Hour), time.Now().Add(-60*24*time.Hour),
	)
	if err != nil {
		t.Fatalf("insert expired: %v", err)
	}

	n, err := auth.SweepExpiredSessions(context.Background(), testDB)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n != 1 {
		t.Fatalf("sweep deleted %d, want 1", n)
	}

	// The valid session should still be there.
	var count int
	if err := testDB.QueryRowContext(context.Background(),
		"SELECT count(*) FROM sessions WHERE id = $1", sess.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatal("valid session was swept away")
	}
}
