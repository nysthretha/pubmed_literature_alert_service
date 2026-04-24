package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/publisher"
)

func NewRouter(db *sql.DB, pub *publisher.Publisher, authCfg auth.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /articles/recent", recentArticlesHandler(db))
	mux.HandleFunc("POST /digest/trigger", triggerDigestHandler(pub))
	auth.RegisterRoutes(mux, authCfg)
	return mux
}
