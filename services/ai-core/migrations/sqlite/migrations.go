// Package sqlite contains the versioned SQLite migrations owned by AI Core.
package sqlite

import _ "embed"

// Initial is the immutable first AI Core SQLite migration.
//
//go:embed 0001_initial.sql
var Initial string
