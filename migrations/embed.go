package migrations

import "embed"

// Files contains the immutable database migration set shipped with this release.
//
//go:embed *.sql
var Files embed.FS
