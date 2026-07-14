// Package sqlite contains the versioned SQLite migrations owned by AI Core.
package sqlite

import _ "embed"

// Initial is the immutable first AI Core SQLite migration.
//
//go:embed 0001_initial.sql
var Initial string

//go:embed 0002_multi_turn_and_tool_correlation.sql
var MultiTurnAndToolCorrelation string

//go:embed 0003_datasource_uid.sql
var DatasourceUID string
