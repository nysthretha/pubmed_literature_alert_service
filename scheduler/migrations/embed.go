// Package migrations exposes the goose migration files as an embedded FS
// so they can be accessed from both the production binary and test helpers.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
