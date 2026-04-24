//go:build !production

// Dev / CI path. No frontend build is required to compile the binary or run
// tests — the SPA is served by Vite at :5173 in dev. The variable still needs
// to exist so the SPA handler can reference it; a zero-value embed.FS reports
// fs.ErrNotExist for every lookup, which the handler treats as "no assets".

package main

import "embed"

var webAssets embed.FS
