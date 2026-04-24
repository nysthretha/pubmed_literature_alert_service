package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
)

// SeedFullSchema migrates to version 5, creates a seed admin (needed so
// migration 00006 can backfill user_id on the queries/digests tables), then
// runs the remaining migrations. Returns the admin user.
//
// Callers will typically follow up with SeedUser() to create non-admin users
// used in isolation tests.
func SeedFullSchema(ctx context.Context, db *sql.DB) (*auth.User, error) {
	if err := MigrateUpTo(db, 5); err != nil {
		return nil, fmt.Errorf("migrate to 5: %w", err)
	}
	admin, err := SeedUser(ctx, db, "admin", "admin@test", "adminpassword123", true)
	if err != nil {
		return nil, fmt.Errorf("seed admin: %w", err)
	}
	if err := MigrateUpTo(db, 0); err != nil {
		return nil, fmt.Errorf("migrate to latest: %w", err)
	}
	return admin, nil
}

// SeedUser inserts a user with a hashed password. Password is hashed inside
// this helper so tests can keep the plaintext for login simulations.
func SeedUser(ctx context.Context, db *sql.DB, username, email, password string, isAdmin bool) (*auth.User, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	return auth.CreateUser(ctx, db, username, email, hash, isAdmin)
}

// SeedSession inserts a session for the given user and returns the cookie
// that tests should attach to requests.
func SeedSession(ctx context.Context, db *sql.DB, userID int64) (*http.Cookie, error) {
	sess, err := auth.CreateSession(ctx, db, userID, "test-agent", "127.0.0.1")
	if err != nil {
		return nil, err
	}
	return &http.Cookie{Name: auth.SessionCookieName, Value: sess.ID}, nil
}
