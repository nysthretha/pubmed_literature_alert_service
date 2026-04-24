package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/apiutil"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/validation"
)

const minPasswordLen = 8

type changePasswordReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (r *changePasswordReq) validate() *validation.ValidationErrors {
	v := validation.New()
	v.Required("current_password", r.CurrentPassword)
	v.Required("new_password", r.NewPassword)
	if r.NewPassword != "" {
		v.MinLen("new_password", r.NewPassword, minPasswordLen)
	}
	return v.Err()
}

// ChangePasswordHandler updates the authenticated user's password.
// Rate-limited via the shared LoginRateLimiter, keyed by IP + username,
// so brute-forcing via change-password is as painful as via login.
func ChangePasswordHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r)
		if !ok {
			apiutil.WriteUnauthorized(w)
			return
		}

		ip := clientIP(r)
		if !cfg.RateLimiter.Allow(ip, u.Username) {
			slog.Warn("auth.change_password.rate_limited", "user_id", u.ID, "ip", ip)
			apiutil.WriteError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
			return
		}

		var req changePasswordReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteBadRequest(w, "invalid JSON")
			return
		}
		if errs := req.validate(); errs != nil {
			apiutil.WriteValidation(w, errs.Fields)
			return
		}

		newHash, err := HashPassword(req.NewPassword)
		if err != nil {
			apiutil.WriteInternal(w, err, "auth.change_password.hash")
			return
		}

		if err := ChangeUserPassword(r.Context(), cfg.DB, u.ID, req.CurrentPassword, newHash); err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				slog.Info("auth.change_password.failed", "reason", "bad_current_password", "user_id", u.ID)
				apiutil.WriteError(w, http.StatusUnauthorized, apiutil.CodeUnauthorized, "current password is incorrect")
				return
			}
			if errors.Is(err, ErrUserNotFound) {
				apiutil.WriteNotFound(w, "user")
				return
			}
			apiutil.WriteInternal(w, err, "auth.change_password")
			return
		}

		slog.Info("auth.change_password.success", "user_id", u.ID)
		w.WriteHeader(http.StatusNoContent)
	}
}
