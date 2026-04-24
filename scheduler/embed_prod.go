//go:build production

// In production the React SPA bundle is copied into scheduler/web/dist by the
// Dockerfile's node stage, then embedded into the binary here. Build with
// `go build -tags=production` to activate this path.

package main

import "embed"

//go:embed all:web/dist
var webAssets embed.FS
