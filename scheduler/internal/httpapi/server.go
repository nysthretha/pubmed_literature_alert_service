package httpapi

import (
	"database/sql"
	"net/http"
)

func NewRouter(db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /articles/recent", recentArticlesHandler(db))
	return mux
}
