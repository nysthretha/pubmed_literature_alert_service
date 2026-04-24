package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// Config bundles handler dependencies.
type Config struct {
	DB           *sql.DB
	RateLimiter  *LoginRateLimiter
	CookieSecure bool
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	User *User `json:"user"`
}

// LoginHandler authenticates a user and issues a session cookie.
// Always returns a generic 401 on any credential failure to avoid leaking
// whether a username exists.
func LoginHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body loginRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		body.Username = strings.TrimSpace(body.Username)
		if body.Username == "" || body.Password == "" {
			writeLoginFailed(w, "missing_fields")
			return
		}

		ip := clientIP(r)
		if !cfg.RateLimiter.Allow(ip, body.Username) {
			slog.Warn("auth.login.failed", "reason", "rate_limited", "username", body.Username, "ip", ip)
			http.Error(w, "too many attempts, try again later", http.StatusTooManyRequests)
			return
		}

		ctx := r.Context()
		u, err := getUserByUsername(ctx, cfg.DB, body.Username)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				slog.Info("auth.login.failed", "reason", "user_not_found", "username", body.Username, "ip", ip)
				writeLoginFailed(w, "generic")
				return
			}
			slog.Error("auth.login.error", "err", err, "username", body.Username, "ip", ip)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		ok, err := VerifyPassword(body.Password, u.PasswordHash)
		if err != nil || !ok {
			slog.Info("auth.login.failed", "reason", "bad_password", "username", body.Username, "ip", ip)
			writeLoginFailed(w, "generic")
			return
		}

		sess, err := CreateSession(ctx, cfg.DB, u.ID, r.UserAgent(), ip)
		if err != nil {
			slog.Error("auth.session.create_failed", "err", err, "user_id", u.ID)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookieName,
			Value:    sess.ID,
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.CookieSecure,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   SessionTTL,
			Expires:  time.Now().Add(time.Duration(SessionTTL) * time.Second),
		})

		slog.Info("auth.login.success",
			"user_id", u.ID, "username", u.Username, "is_admin", u.IsAdmin,
			"sess_id", LogPrefix(sess.ID), "ip", ip,
		)

		writeJSON(w, http.StatusOK, loginResponse{User: u.toPublic()})
	}
}

// LogoutHandler invalidates the current session and clears the cookie.
// Safe to call without a session (returns 200 regardless).
func LogoutHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
			if err := DeleteSession(r.Context(), cfg.DB, c.Value); err != nil {
				slog.Error("auth.logout.delete_failed", "err", err)
			} else {
				slog.Info("auth.logout", "sess_id", LogPrefix(c.Value))
			}
		}

		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.CookieSecure,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})

		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	}
}

// MeHandler returns the authenticated user's public shape.
// Assumes the Required middleware has run and injected the user into context.
func MeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": u})
	}
}

// --- helpers ---

func writeLoginFailed(w http.ResponseWriter, _reason string) {
	// Same response regardless of reason (user-not-found vs bad-password vs missing-fields)
	// so attackers can't distinguish. Reason is logged server-side only.
	http.Error(w, "invalid credentials", http.StatusUnauthorized)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// clientIP returns a best-effort client IP. For single-instance localhost this
// is r.RemoteAddr's host portion. If a reverse proxy is added later, parse
// X-Forwarded-For here.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
