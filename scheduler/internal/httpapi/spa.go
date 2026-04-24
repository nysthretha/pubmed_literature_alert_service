package httpapi

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"strings"
)

// spaHandler serves the embedded React SPA build with fall-through routing:
//
//   - GET / → index.html
//   - GET /assets/xyz.js → the embedded asset (if present)
//   - GET /queries (client-side route) → index.html, letting the SPA router
//     pick up the location
//   - GET /api/* → NOT hit here, the mux routes those first
//   - GET /healthz → NOT hit here, ditto
//
// In dev builds the embedded FS is empty (see embed_dev.go); every request
// falls through to 404. That's fine because Vite at :5173 serves the SPA in
// dev and proxies only /api/* to the Go scheduler.
func spaHandler(assets embed.FS, subPath string) http.HandlerFunc {
	sub, err := fs.Sub(assets, subPath)
	// If the embed is empty (dev build), fs.Sub returns a valid but empty FS —
	// Open("index.html") errors out below and we 404.
	if err != nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "spa not available", http.StatusNotFound)
		}
	}

	fileServer := http.FileServer(http.FS(sub))

	return func(w http.ResponseWriter, r *http.Request) {
		// Trim the leading slash for fs.Open consistency.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// If a real file exists at this path, serve it directly.
		if f, err := sub.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		} else if !errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "io error", http.StatusInternalServerError)
			return
		}

		// Path missing — fall through to index.html for client-side routes.
		if idx, err := sub.Open("index.html"); err == nil {
			_ = idx.Close()
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Dev build (empty embed) or malformed dist — 404.
		http.NotFound(w, r)
	}
}
