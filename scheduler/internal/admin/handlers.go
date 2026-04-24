// Package admin implements the admin-only HTTP endpoints. Handlers call
// the Admin* methods exposed by the auth package.
//
// All routes are wrapped in auth.Required + auth.AdminRequired — see
// RegisterRoutes. Non-admin users reach 403 before hitting these handlers.
package admin

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/apiutil"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/validation"
)

const minAdminPasswordLen = 12

var emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// --- GET /api/admin/users ---

func listUsersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, err := apiutil.ParseLimitOffset(r, 50, 200)
		if err != nil {
			apiutil.WriteBadRequest(w, err.Error())
			return
		}
		list, total, err := auth.AdminListUsers(r.Context(), db, limit, offset)
		if err != nil {
			apiutil.WriteInternal(w, err, "admin.users.list")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"users":    list,
			"total":    total,
			"has_more": offset+len(list) < total,
		})
	}
}

// --- POST /api/admin/users ---

type createUserReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"is_admin,omitempty"` // admins can create other admins
}

func (r *createUserReq) Validate() *validation.ValidationErrors {
	v := validation.New()
	v.Required("username", r.Username)
	v.MinLen("username", r.Username, 3)
	v.MaxLen("username", r.Username, 40)
	v.Required("email", r.Email)
	v.MaxLen("email", r.Email, 255)
	if r.Email != "" {
		v.Matches("email", r.Email, emailRE, "a valid email address")
	}
	v.MinLen("password", r.Password, minAdminPasswordLen)
	return v.Err()
}

func createUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createUserReq
		if !apiutil.DecodeJSON(w, r, &req) {
			return
		}
		if errs := req.Validate(); errs != nil {
			apiutil.WriteValidation(w, errs.Fields)
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			apiutil.WriteInternal(w, err, "admin.users.hash")
			return
		}
		user, err := auth.AdminCreateUser(r.Context(), db, req.Username, req.Email, hash, req.IsAdmin)
		if err != nil {
			if errors.Is(err, auth.ErrUserExists) {
				apiutil.WriteConflict(w, "a user with that username or email already exists")
				return
			}
			apiutil.WriteInternal(w, err, "admin.users.create")
			return
		}
		actor, _ := auth.UserFromContext(r)
		slog.Info("admin.users.create",
			"admin_id", actor.ID, "created_user_id", user.ID,
			"username", user.Username, "is_admin", user.IsAdmin,
		)
		apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"user": user})
	}
}

// --- PATCH /api/admin/users/:id ---

type updateUserReq struct {
	Email   *string `json:"email,omitempty"`
	IsAdmin *bool   `json:"is_admin,omitempty"`
}

func (r *updateUserReq) Validate() *validation.ValidationErrors {
	v := validation.New()
	if r.Email != nil {
		v.Required("email", *r.Email)
		v.MaxLen("email", *r.Email, 255)
		if *r.Email != "" {
			v.Matches("email", *r.Email, emailRE, "a valid email address")
		}
	}
	return v.Err()
}

func updateUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseIDPath(w, r)
		if !ok {
			return
		}
		var req updateUserReq
		if !apiutil.DecodeJSON(w, r, &req) {
			return
		}
		if errs := req.Validate(); errs != nil {
			apiutil.WriteValidation(w, errs.Fields)
			return
		}
		u, err := auth.AdminUpdateUser(r.Context(), db, id, req.Email, req.IsAdmin)
		if err != nil {
			if errors.Is(err, auth.ErrUserNotFound) {
				apiutil.WriteNotFound(w, "user")
				return
			}
			if errors.Is(err, auth.ErrUserExists) {
				apiutil.WriteConflict(w, "a user with that email already exists")
				return
			}
			apiutil.WriteInternal(w, err, "admin.users.update")
			return
		}
		actor, _ := auth.UserFromContext(r)
		slog.Info("admin.users.update", "admin_id", actor.ID, "user_id", u.ID)
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{"user": u})
	}
}

// --- DELETE /api/admin/users/:id ---

func deleteUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseIDPath(w, r)
		if !ok {
			return
		}
		actor, _ := auth.UserFromContext(r)
		if actor != nil && actor.ID == id {
			apiutil.WriteBadRequest(w, "cannot delete your own account via admin endpoint")
			return
		}

		if err := auth.AdminDeleteUser(r.Context(), db, id); err != nil {
			if errors.Is(err, auth.ErrUserNotFound) {
				apiutil.WriteNotFound(w, "user")
				return
			}
			apiutil.WriteInternal(w, err, "admin.users.delete")
			return
		}
		slog.Info("admin.users.delete", "admin_id", actor.ID, "deleted_user_id", id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- POST /api/admin/users/:id/reset-password ---

type resetPasswordReq struct {
	NewPassword string `json:"new_password"`
}

func (r *resetPasswordReq) Validate() *validation.ValidationErrors {
	v := validation.New()
	v.MinLen("new_password", r.NewPassword, minAdminPasswordLen)
	return v.Err()
}

func resetPasswordHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseIDPath(w, r)
		if !ok {
			return
		}
		var req resetPasswordReq
		if !apiutil.DecodeJSON(w, r, &req) {
			return
		}
		if errs := req.Validate(); errs != nil {
			apiutil.WriteValidation(w, errs.Fields)
			return
		}
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			apiutil.WriteInternal(w, err, "admin.users.reset.hash")
			return
		}
		if err := auth.AdminResetPassword(r.Context(), db, id, hash); err != nil {
			if errors.Is(err, auth.ErrUserNotFound) {
				apiutil.WriteNotFound(w, "user")
				return
			}
			apiutil.WriteInternal(w, err, "admin.users.reset")
			return
		}
		actor, _ := auth.UserFromContext(r)
		slog.Info("admin.users.reset_password", "admin_id", actor.ID, "user_id", id)
		apiutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "password_reset"})
	}
}

// --- helpers ---

func parseIDPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		apiutil.WriteBadRequest(w, "invalid id in path")
		return 0, false
	}
	return id, true
}
