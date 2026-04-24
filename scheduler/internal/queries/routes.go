package queries

import (
	"database/sql"
	"net/http"
)

// RegisterRoutes installs the query CRUD endpoints on mux, wrapping each
// with the provided authRequired middleware (auth.Required).
func RegisterRoutes(mux *http.ServeMux, db *sql.DB, authRequired func(http.Handler) http.Handler) {
	mux.Handle("GET /api/queries", authRequired(listHandler(db)))
	mux.Handle("POST /api/queries", authRequired(createHandler(db)))
	mux.Handle("GET /api/queries/{id}", authRequired(getHandler(db)))
	mux.Handle("PATCH /api/queries/{id}", authRequired(patchHandler(db)))
	mux.Handle("DELETE /api/queries/{id}", authRequired(deleteHandler(db)))
	mux.Handle("POST /api/queries/{id}/repoll", authRequired(repollHandler(db)))
}
