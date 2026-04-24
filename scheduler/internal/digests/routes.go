package digests

import (
	"database/sql"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB, authRequired func(http.Handler) http.Handler) {
	mux.Handle("GET /api/digests", authRequired(listHandler(db)))
	mux.Handle("GET /api/digests/{id}", authRequired(getHandler(db)))
}
