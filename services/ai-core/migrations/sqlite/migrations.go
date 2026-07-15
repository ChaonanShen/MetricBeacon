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

//go:embed 0004_bounded_query_plan.sql
var BoundedQueryPlan string

//go:embed 0005_query_plan_views.sql
var QueryPlanViews string

//go:embed 0006_session_history_index.sql
var SessionHistoryIndex string

//go:embed 0007_incident_task_union.sql
var IncidentTaskUnion string

//go:embed 0008_remediation_lifecycle.sql
var RemediationLifecycle string
