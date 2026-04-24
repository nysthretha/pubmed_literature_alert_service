package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// Session is the minimal session shape; the id is the cookie value.
type Session struct {
	ID         string
	UserID     int64
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

// NewSessionID returns a base64-url-encoded random 32-byte session identifier.
func NewSessionID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("session id rng: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// CreateSession inserts a fresh session for a user with the absolute TTL.
func CreateSession(ctx context.Context, db *sql.DB, userID int64, userAgent, ip string) (*Session, error) {
	id, err := NewSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	expires := now.Add(time.Duration(SessionTTL) * time.Second)

	var ua, ipArg any
	if userAgent == "" {
		ua = nil
	} else {
		ua = userAgent
	}
	if ip == "" {
		ipArg = nil
	} else {
		ipArg = ip
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, created_at, expires_at, last_seen_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, $3, $5, $6)
	`, id, userID, now, expires, ua, ipArg)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return &Session{
		ID: id, UserID: userID,
		CreatedAt: now, ExpiresAt: expires, LastSeenAt: now,
	}, nil
}

// lookupSession joins sessions + users, validates expiry + idle timeout, and
// updates last_seen_at in the same round-trip when valid. Returns
// ErrSessionNotFound / ErrSessionExpired as appropriate.
func lookupSession(ctx context.Context, db *sql.DB, sessionID string) (*User, *Session, error) {
	if sessionID == "" {
		return nil, nil, ErrSessionNotFound
	}

	var (
		sess Session
		u    User
		now  = time.Now().UTC()
	)
	err := db.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, s.created_at, s.expires_at, s.last_seen_at,
		       u.id, u.username, u.email, u.is_admin
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1
	`, sessionID).Scan(
		&sess.ID, &sess.UserID, &sess.CreatedAt, &sess.ExpiresAt, &sess.LastSeenAt,
		&u.ID, &u.Username, &u.Email, &u.IsAdmin,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	if now.After(sess.ExpiresAt) {
		return nil, nil, ErrSessionExpired
	}
	idle := time.Duration(SessionIdleTimeout) * time.Second
	if now.Sub(sess.LastSeenAt) > idle {
		return nil, nil, ErrSessionExpired
	}

	// Update last_seen_at.
	if _, err := db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = $1 WHERE id = $2`, now, sess.ID); err != nil {
		return nil, nil, fmt.Errorf("touch session: %w", err)
	}
	sess.LastSeenAt = now

	return &u, &sess, nil
}

// DeleteSession removes a session by id. Idempotent.
func DeleteSession(ctx context.Context, db *sql.DB, sessionID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}

// SweepExpiredSessions deletes sessions past the absolute TTL or the idle timeout.
// Returns the number of rows deleted.
func SweepExpiredSessions(ctx context.Context, db *sql.DB) (int64, error) {
	res, err := db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE expires_at < now()
		   OR last_seen_at < now() - make_interval(secs => $1)
	`, SessionIdleTimeout)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// LogPrefix returns the first 8 characters of a session ID for safe logging.
func LogPrefix(sessionID string) string {
	if len(sessionID) < 8 {
		return sessionID
	}
	return sessionID[:8] + "..."
}
