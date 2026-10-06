package migrations

import "embed"

//go:embed sqlite/*.up.sql
var Files embed.FS
