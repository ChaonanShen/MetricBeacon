package sqlite

import (
	"context"
	"database/sql"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	migrations "mini-torchbearing.local/services/ai-core/migrations/sqlite"
)

type migration struct {
	version int
	sql     string
	before  func(context.Context, *sql.Tx) error
	after   func(context.Context, *sql.Tx) error
}

var allMigrations = []migration{
	{version: 1, sql: migrations.Initial},
	{version: 2, sql: migrations.MultiTurnAndToolCorrelation, before: validateMultiTurnPreconditions, after: validateMultiTurnPostconditions},
	{version: 3, sql: migrations.DatasourceUID},
	{version: 4, sql: migrations.BoundedQueryPlan},
	{version: 5, sql: migrations.QueryPlanViews},
	{version: 6, sql: migrations.SessionHistoryIndex},
	{version: 7, sql: migrations.IncidentTaskUnion, after: validateIncidentTaskUnion},
	{version: 8, sql: migrations.RemediationLifecycle, after: validateRemediationLifecycle},
}

func validateRemediationLifecycle(ctx context.Context, tx *sql.Tx) error {
	checks := []string{
		`SELECT 1 FROM pragma_foreign_key_check LIMIT 1`,
		`SELECT 1 FROM remediation_intents i LEFT JOIN tasks t ON t.id = i.task_id AND t.tenant_id = i.tenant_id AND t.kind = 'incident_remediation' WHERE t.id IS NULL LIMIT 1`,
		`SELECT 1 FROM approvals a LEFT JOIN remediation_intents i ON i.id = a.intent_id AND i.tenant_id = a.tenant_id AND i.task_id = a.task_id AND i.digest = a.intent_digest WHERE i.id IS NULL LIMIT 1`,
	}
	for _, query := range checks {
		var found any
		err := tx.QueryRowContext(ctx, query).Scan(&found)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return mapError(err)
		}
		return common.NewError(common.InternalError, "remediation lifecycle migration validation failed", false)
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return mapError(err)
	}
	for _, item := range allMigrations {
		var applied int
		err := tx.QueryRowContext(ctx, "SELECT 1 FROM schema_migrations WHERE version = ?", item.version).Scan(&applied)
		switch {
		case err == nil:
			continue
		case err != sql.ErrNoRows:
			return mapError(err)
		}
		if item.before != nil {
			if err := item.before(ctx, tx); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, item.sql); err != nil {
			return mapError(err)
		}
		if item.after != nil {
			if err := item.after(ctx, tx); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", item.version, storageTimestamp(nowUTC())); err != nil {
			return mapError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return mapError(err)
	}
	return nil
}

func nowUTC() time.Time { return time.Now().UTC() }

func validateMultiTurnPreconditions(ctx context.Context, tx *sql.Tx) error {
	checks := []string{
		`SELECT 1 FROM tasks t LEFT JOIN messages m ON m.id = t.input_message_id AND m.tenant_id = t.tenant_id AND m.session_id = t.session_id WHERE m.id IS NULL OR m.role != 'user' LIMIT 1`,
		`SELECT 1 FROM (SELECT tenant_id, input_message_id, COUNT(*) AS total FROM tasks GROUP BY tenant_id, input_message_id HAVING total != 1) LIMIT 1`,
		`SELECT 1 FROM messages m WHERE m.role = 'user' AND (SELECT COUNT(*) FROM tasks t WHERE t.tenant_id = m.tenant_id AND t.session_id = m.session_id AND t.input_message_id = m.id) != 1 LIMIT 1`,
		`SELECT 1 FROM messages m WHERE m.role = 'assistant' AND (SELECT COUNT(*) FROM task_events e WHERE e.tenant_id = m.tenant_id AND e.type = 'assistant.message.completed' AND json_extract(e.payload_json, '$.message.id') = m.id) != 1 LIMIT 1`,
		`SELECT 1 FROM (SELECT tenant_id, session_id, COUNT(*) AS total FROM tasks WHERE status NOT IN ('completed', 'failed') GROUP BY tenant_id, session_id HAVING total > 1) LIMIT 1`,
	}
	for _, query := range checks {
		var found int
		err := tx.QueryRowContext(ctx, query).Scan(&found)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return mapError(err)
		}
		return common.NewError(common.InvalidStateTransition, "cannot migrate ambiguous task/message data", false)
	}
	return nil
}

func validateMultiTurnPostconditions(ctx context.Context, tx *sql.Tx) error {
	checks := []string{
		`SELECT 1 FROM messages WHERE task_id IS NULL OR task_id = '' LIMIT 1`,
		`SELECT 1 FROM tasks t LEFT JOIN messages m ON m.id = t.input_message_id AND m.tenant_id = t.tenant_id AND m.session_id = t.session_id AND m.task_id = t.id AND m.role = 'user' WHERE m.id IS NULL LIMIT 1`,
		`SELECT 1 FROM messages m LEFT JOIN tasks t ON t.id = m.task_id AND t.tenant_id = m.tenant_id AND t.session_id = m.session_id WHERE t.id IS NULL LIMIT 1`,
		`SELECT 1 FROM tool_calls WHERE source_call_id IS NULL OR source_call_id = '' LIMIT 1`,
		`SELECT 1 FROM pragma_foreign_key_check LIMIT 1`,
	}
	for _, query := range checks {
		var found any
		err := tx.QueryRowContext(ctx, query).Scan(&found)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return mapError(err)
		}
		return common.NewError(common.InternalError, "multi-turn migration validation failed", false)
	}
	return nil
}

func validateIncidentTaskUnion(ctx context.Context, tx *sql.Tx) error {
	checks := []string{
		`SELECT 1 FROM sessions WHERE org_id = '' OR kind NOT IN ('private', 'org_incident') LIMIT 1`,
		`SELECT 1 FROM tasks WHERE kind != 'metric_analysis' OR incident_plan_json IS NOT NULL LIMIT 1`,
		`SELECT 1 FROM messages WHERE role NOT IN ('user', 'assistant', 'trigger') LIMIT 1`,
		`SELECT 1 FROM pragma_foreign_key_check LIMIT 1`,
	}
	for _, query := range checks {
		var found any
		err := tx.QueryRowContext(ctx, query).Scan(&found)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return mapError(err)
		}
		return common.NewError(common.InternalError, "incident task migration validation failed", false)
	}
	return nil
}
