package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/admin"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/articles"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/digests"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/publisher"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/queries"
)

func NewRouter(db *sql.DB, pub *publisher.Publisher, authCfg auth.Config) http.Handler {
	mux := http.NewServeMux()

	// Auth endpoints (login/logout/me). Public.
	auth.RegisterRoutes(mux, authCfg)

	// Digest manual-trigger (localhost-bound via compose port mapping).
	// Left public for now; the only callers are the user or a local script.
	mux.HandleFunc("POST /digest/trigger", triggerDigestHandler(pub))

	// Middleware chains used by M5b resource routes.
	authRequired := auth.Required(db)
	adminRequired := func(next http.Handler) http.Handler {
		return authRequired(auth.AdminRequired(next))
	}

	// User-scoped CRUD/read endpoints.
	queries.RegisterRoutes(mux, db, authRequired)
	articles.RegisterRoutes(mux, db, authRequired)
	digests.RegisterRoutes(mux, db, authRequired)

	// Admin-only endpoints.
	admin.RegisterRoutes(mux, db, adminRequired)

	return mux
}
