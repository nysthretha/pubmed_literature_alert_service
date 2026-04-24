package articles

import (
	"database/sql"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB, authRequired func(http.Handler) http.Handler) {
	mux.Handle("GET /api/articles", authRequired(listHandler(db)))
	mux.Handle("GET /api/articles/{pmid}", authRequired(getHandler(db)))
}
