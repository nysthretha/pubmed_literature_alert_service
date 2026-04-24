// Package auth provides authentication: users, password hashing (argon2id),
// sessions persisted in Postgres, login/logout/me HTTP handlers, and middleware.
//
// CSRF protection is omitted intentionally — the API lives at a single origin,
// issues cookies with SameSite=Strict, has no third-party embedding, and has
// no public unauthenticated mutating endpoints. Modern browsers block CSRF in
// this architecture. If any of those assumptions change, add CSRF tokens then.
package auth

import (
	"errors"
	"net/http"
)

// SessionCookieName is the cookie key used for session IDs.
const SessionCookieName = "pubmed_session"

// SessionTTL is the absolute TTL for a session.
const SessionTTL = 30 * 24 * 60 * 60 // seconds (30 days)

// SessionIdleTimeout is the idle-timeout after which a session is considered expired.
const SessionIdleTimeout = 7 * 24 * 60 * 60 // seconds (7 days)

// User is the minimal user shape exposed through handlers and context.
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	IsAdmin   bool   `json:"is_admin"`
}

// ctxKey is a private type for request-context keys.
type ctxKey int

const (
	ctxKeyUser    ctxKey = 1
	ctxKeySession ctxKey = 2
)

// Errors returned by auth operations.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session expired")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrRateLimited        = errors.New("rate limited")
)

// UserFromContext returns the authenticated user from the request context, if any.
func UserFromContext(r *http.Request) (*User, bool) {
	u, ok := r.Context().Value(ctxKeyUser).(*User)
	return u, ok
}

// SessionIDFromContext returns the session ID from the request context, if any.
func SessionIDFromContext(r *http.Request) (string, bool) {
	s, ok := r.Context().Value(ctxKeySession).(string)
	return s, ok
}
