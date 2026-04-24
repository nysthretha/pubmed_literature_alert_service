package admin

import (
	"database/sql"
	"net/http"
)

// RegisterRoutes installs admin-only endpoints. The caller passes a middleware
// chain that already includes auth.Required + auth.AdminRequired.
func RegisterRoutes(mux *http.ServeMux, db *sql.DB, adminRequired func(http.Handler) http.Handler) {
	mux.Handle("GET /api/admin/users", adminRequired(listUsersHandler(db)))
	mux.Handle("POST /api/admin/users", adminRequired(createUserHandler(db)))
	mux.Handle("PATCH /api/admin/users/{id}", adminRequired(updateUserHandler(db)))
	mux.Handle("DELETE /api/admin/users/{id}", adminRequired(deleteUserHandler(db)))
	mux.Handle("POST /api/admin/users/{id}/reset-password", adminRequired(resetPasswordHandler(db)))
}
