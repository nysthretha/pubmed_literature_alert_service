package auth

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
)

// Required returns a middleware that requires a valid session cookie.
// On success, the user and session id are injected into the request context.
func Required(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(SessionCookieName)
			if err != nil || c.Value == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			user, sess, err := lookupSession(r.Context(), db, c.Value)
			if err != nil {
				if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionExpired) {
					slog.Info("auth.session.expired", "sess_id", LogPrefix(c.Value), "reason", err.Error())
					clearCookie(w)
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				slog.Error("auth.session.lookup_failed", "err", err, "sess_id", LogPrefix(c.Value))
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUser, user)
			ctx = context.WithValue(ctx, ctxKeySession, sess.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminRequired layers on top of Required: 403 if user.is_admin is false.
// Must be composed after Required.
func AdminRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !u.IsAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clearCookie issues a cookie with an immediate expiry so the browser drops it.
func clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
