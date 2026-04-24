package httpapi

import (
	"database/sql"
	"embed"
	"net/http"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/admin"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/articles"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/digests"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/publisher"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/queries"
)

// NewRouter assembles the full HTTP router. webAssets is the embedded SPA
// bundle (empty in dev builds); webAssetsSubPath is the directory within the
// embed FS that contains the dist/ output (e.g. "web/dist").
func NewRouter(
	db *sql.DB,
	pub *publisher.Publisher,
	authCfg auth.Config,
	webAssets embed.FS,
	webAssetsSubPath string,
) http.Handler {
	mux := http.NewServeMux()

	// Unauthenticated operational endpoints.
	mux.HandleFunc("GET /healthz", healthzHandler(db, pub))

	// Auth endpoints (login/logout/me/change-password).
	auth.RegisterRoutes(mux, authCfg)

	// Digest manual trigger (bound to 127.0.0.1 in dev; publicly reachable in
	// prod but benign — the only effect is enqueuing a trigger for the digest
	// worker, which runs in the user's own account context).
	mux.HandleFunc("POST /digest/trigger", triggerDigestHandler(pub))

	// Middleware chains for user and admin routes.
	authRequired := auth.Required(db)
	adminRequired := func(next http.Handler) http.Handler {
		return authRequired(auth.AdminRequired(next))
	}

	// User-scoped CRUD/read endpoints.
	queries.RegisterRoutes(mux, db, authRequired)
	articles.RegisterRoutes(mux, db, authRequired)
	digests.RegisterRoutes(mux, db, authRequired)
	admin.RegisterRoutes(mux, db, adminRequired)

	// SPA fallback — anything that didn't match above. The fallback handler
	// serves embedded assets or index.html with SPA routing semantics.
	mux.Handle("GET /", spaHandler(webAssets, webAssetsSubPath))

	return mux
}
