package auth

import "net/http"

// RegisterRoutes installs the /api/auth/* endpoints on the given mux.
// GET /api/auth/me is wrapped in Required middleware; the others are public.
func RegisterRoutes(mux *http.ServeMux, cfg Config) {
	mux.Handle("POST /api/auth/login", LoginHandler(cfg))
	mux.Handle("POST /api/auth/logout", LogoutHandler(cfg))
	mux.Handle("GET /api/auth/me", Required(cfg.DB)(MeHandler()))
}
