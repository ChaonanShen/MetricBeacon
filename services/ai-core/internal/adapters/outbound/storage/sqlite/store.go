package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

const storageTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"
const maxReplayBatch = 200

var _ repositories.ApplicationStore = (*Store)(nil)

type queryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type transactionState struct {
	mu     sync.RWMutex
	active bool
}

// Store exposes only repository ports. The sql.DB/sql.Tx never leaves this
// adapter package, including while callers use WithinTransaction.
type Store struct {
	db      *sql.DB
	tx      *sql.Tx
	txState *transactionState
}

func (s *Store) Sessions() repositories.SessionRepository   { return sessionRepository{store: s} }
func (s *Store) Messages() repositories.MessageRepository   { return messageRepository{store: s} }
func (s *Store) Tasks() repositories.TaskRepository         { return taskRepository{store: s} }
func (s *Store) ToolCalls() repositories.ToolCallRepository { return toolCallRepository{store: s} }
func (s *Store) Charts() repositories.ChartRepository       { return chartRepository{store: s} }
func (s *Store) ChartExecutions() repositories.ChartExecutionRepository {
	return chartExecutionRepository{store: s}
}
func (s *Store) Idempotency() repositories.IdempotencyRepository {
	return idempotencyRepository{store: s}
}
func (s *Store) TaskEvents() events.Store { return eventStore{store: s} }

type sessionRepository struct{ store *Store }

func (r sessionRepository) Create(ctx context.Context, value session.AnalysisSession) error {
	return r.store.createSession(ctx, value)
}
func (r sessionRepository) Get(ctx context.Context, tenantID, sessionID string) (session.AnalysisSession, error) {
	return r.store.getSession(ctx, tenantID, sessionID)
}
func (r sessionRepository) Update(ctx context.Context, value session.AnalysisSession, expectedVersion int64) error {
	return r.store.updateSession(ctx, value, expectedVersion)
}

type messageRepository struct{ store *Store }

func (r messageRepository) Append(ctx context.Context, value session.Message) error {
	return r.store.appendMessage(ctx, value)
}
func (r messageRepository) ListBySession(ctx context.Context, tenantID, sessionID string) ([]session.Message, error) {
	return r.store.listMessagesBySession(ctx, tenantID, sessionID)
}

type taskRepository struct{ store *Store }

func (r taskRepository) Create(ctx context.Context, value task.AnalysisTask) error {
	return r.store.createTask(ctx, value)
}
func (r taskRepository) Get(ctx context.Context, tenantID, taskID string) (task.AnalysisTask, error) {
	return r.store.getTask(ctx, tenantID, taskID)
}
func (r taskRepository) Update(ctx context.Context, value task.AnalysisTask, expectedVersion int64) error {
	return r.store.updateTask(ctx, value, expectedVersion)
}

type toolCallRepository struct{ store *Store }

func (r toolCallRepository) Create(ctx context.Context, value task.ToolCallRecord) error {
	return r.store.createToolCall(ctx, value)
}
func (r toolCallRepository) Complete(ctx context.Context, value task.ToolCallRecord, expectedVersion int64) error {
	return r.store.completeToolCall(ctx, value, expectedVersion)
}
func (r toolCallRepository) ListByTask(ctx context.Context, tenantID, taskID string) ([]task.ToolCallRecord, error) {
	return r.store.listToolCallsByTask(ctx, tenantID, taskID)
}

type chartRepository struct{ store *Store }

func (r chartRepository) Create(ctx context.Context, value chart.ChartDraft) error {
	return r.store.createChart(ctx, value)
}
func (r chartRepository) Get(ctx context.Context, tenantID, chartID string) (chart.ChartDraft, error) {
	return r.store.getChart(ctx, tenantID, chartID)
}
func (r chartRepository) Update(ctx context.Context, value chart.ChartDraft, expectedVersion int64) error {
	return r.store.updateChart(ctx, value, expectedVersion)
}
func (r chartRepository) ListByTask(ctx context.Context, tenantID, taskID string) ([]chart.ChartDraft, error) {
	return r.store.listChartsByTask(ctx, tenantID, taskID)
}

type chartExecutionRepository struct{ store *Store }

func (r chartExecutionRepository) Create(ctx context.Context, value chart.Execution) error {
	return r.store.createChartExecution(ctx, value)
}
func (r chartExecutionRepository) ListByChart(ctx context.Context, tenantID, chartID string) ([]chart.Execution, error) {
	return r.store.listChartExecutionsByChart(ctx, tenantID, chartID)
}

type idempotencyRepository struct{ store *Store }

func (r idempotencyRepository) Reserve(ctx context.Context, key repositories.IdempotencyKey, requestHash string, expiresAt time.Time) (repositories.IdempotencyRecord, error) {
	return r.store.reserve(ctx, key, requestHash, expiresAt)
}
func (r idempotencyRepository) GetResult(ctx context.Context, key repositories.IdempotencyKey) (repositories.IdempotencyRecord, error) {
	return r.store.getIdempotencyResult(ctx, key)
}
func (r idempotencyRepository) Complete(ctx context.Context, key repositories.IdempotencyKey, resourceID string, responseJSON []byte) error {
	return r.store.completeIdempotency(ctx, key, resourceID, responseJSON)
}

type eventStore struct{ store *Store }

func (r eventStore) Append(ctx context.Context, draft task.EventDraft) (task.TaskEvent, error) {
	return r.store.appendEvent(ctx, draft)
}
func (r eventStore) Replay(ctx context.Context, tenantID, taskID string, afterSequence int64, limit int) ([]task.TaskEvent, error) {
	return r.store.replayEvents(ctx, tenantID, taskID, afterSequence, limit)
}
func (r eventStore) LatestSequence(ctx context.Context, tenantID, taskID string) (int64, error) {
	return r.store.latestSequence(ctx, tenantID, taskID)
}

func (s *Store) WithinTransaction(ctx context.Context, fn func(repositories.ApplicationStore) error) error {
	if fn == nil {
		return common.NewError(common.InvalidArgument, "transaction callback is required", false)
	}
	if err := s.ensureActive(); err != nil {
		return err
	}
	if s.tx != nil {
		return fn(s)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapError(err)
	}
	state := &transactionState{active: true}
	txStore := &Store{db: s.db, tx: tx, txState: state}
	if err := fn(txStore); err != nil {
		_ = tx.Rollback()
		state.deactivate()
		return mapError(err)
	}
	if err := tx.Commit(); err != nil {
		state.deactivate()
		return mapError(err)
	}
	state.deactivate()
	return nil
}

func (s *Store) Health(ctx context.Context) error {
	if err := s.ensureActive(); err != nil {
		return err
	}
	var foreignKeys int
	if err := s.executor().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return mapError(err)
	}
	if foreignKeys != 1 {
		return common.NewError(common.InternalError, "SQLite foreign keys are not enabled", false)
	}
	return nil
}

func (s *Store) Close() error {
	if s.db == nil || s.tx != nil {
		return nil
	}
	return mapError(s.db.Close())
}

func (s *Store) createSession(ctx context.Context, value session.AnalysisSession) error {
	if value.ID == "" || value.TenantID == "" || value.CreatedBy == "" || strings.TrimSpace(value.Title) == "" || value.Status != session.StatusActive || value.Version < 1 {
		return common.NewError(common.InvalidArgument, "session is invalid", false)
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO sessions (id, tenant_id, title, status, created_by, created_at, updated_at, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.Title, value.Status, value.CreatedBy, storageTimestamp(value.CreatedAt), storageTimestamp(value.UpdatedAt), value.Version)
	return mapError(err)
}

func (s *Store) getSession(ctx context.Context, tenantID, sessionID string) (session.AnalysisSession, error) {
	if tenantID == "" || sessionID == "" {
		return session.AnalysisSession{}, common.NewError(common.InvalidArgument, "tenant and session are required", false)
	}
	row := s.executor().QueryRowContext(ctx, `SELECT id, tenant_id, title, status, created_by, created_at, updated_at, version FROM sessions WHERE tenant_id = ? AND id = ?`, tenantID, sessionID)
	return scanSession(row)
}

func (s *Store) updateSession(ctx context.Context, value session.AnalysisSession, expectedVersion int64) error {
	if value.ID == "" || value.TenantID == "" || expectedVersion < 1 || value.Version != expectedVersion+1 || value.Status != session.StatusActive {
		return common.NewError(common.InvalidArgument, "session update is invalid", false)
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE sessions SET title = ?, status = ?, updated_at = ?, version = ? WHERE tenant_id = ? AND id = ? AND version = ?`, value.Title, value.Status, storageTimestamp(value.UpdatedAt), value.Version, value.TenantID, value.ID, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "session")
}

func (s *Store) appendMessage(ctx context.Context, value session.Message) error {
	if value.ID == "" || value.TenantID == "" || value.SessionID == "" || strings.TrimSpace(value.Content) == "" || (value.Role != session.RoleUser && value.Role != session.RoleAssistant) {
		return common.NewError(common.InvalidArgument, "message is invalid", false)
	}
	if err := s.ensureSession(ctx, value.TenantID, value.SessionID); err != nil {
		return err
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO messages (id, tenant_id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.SessionID, value.Role, value.Content, storageTimestamp(value.CreatedAt))
	return mapError(err)
}

func (s *Store) listMessagesBySession(ctx context.Context, tenantID, sessionID string) ([]session.Message, error) {
	if tenantID == "" || sessionID == "" {
		return nil, common.NewError(common.InvalidArgument, "tenant and session are required", false)
	}
	if err := s.ensureSession(ctx, tenantID, sessionID); err != nil {
		return nil, err
	}
	rows, err := s.executor().QueryContext(ctx, `SELECT id, tenant_id, session_id, role, content, created_at FROM messages WHERE tenant_id = ? AND session_id = ? ORDER BY created_at, id`, tenantID, sessionID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	messages := make([]session.Message, 0)
	for rows.Next() {
		var item session.Message
		var createdAt string
		if err := rows.Scan(&item.ID, &item.TenantID, &item.SessionID, &item.Role, &item.Content, &createdAt); err != nil {
			return nil, mapError(err)
		}
		parsed, err := parseTimestamp(createdAt)
		if err != nil {
			return nil, err
		}
		item.CreatedAt = parsed
		messages = append(messages, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return messages, nil
}

func (s *Store) createTask(ctx context.Context, value task.AnalysisTask) error {
	if err := validateTask(value); err != nil {
		return err
	}
	if err := s.ensureSession(ctx, value.TenantID, value.SessionID); err != nil {
		return err
	}
	if err := s.ensureMessage(ctx, value.TenantID, value.SessionID, value.InputMessageID); err != nil {
		return err
	}
	var errorCode, errorMessage any
	if value.Error != nil {
		errorCode, errorMessage = string(value.Error.Code), value.Error.Message
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO tasks (id, tenant_id, session_id, status, input_message_id, datasource_uid, time_from, time_to, latest_sequence, error_code, error_message, created_at, started_at, completed_at, updated_at, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.SessionID, value.Status, value.InputMessageID, value.DatasourceUID, storageTimestamp(value.TimeRange.From), storageTimestamp(value.TimeRange.To), value.LatestSequence, errorCode, errorMessage, storageTimestamp(value.CreatedAt), nullableTimestamp(value.StartedAt), nullableTimestamp(value.CompletedAt), storageTimestamp(value.UpdatedAt), value.Version)
	return mapError(err)
}

func (s *Store) getTask(ctx context.Context, tenantID, taskID string) (task.AnalysisTask, error) {
	if tenantID == "" || taskID == "" {
		return task.AnalysisTask{}, common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	row := s.executor().QueryRowContext(ctx, `SELECT id, tenant_id, session_id, status, input_message_id, datasource_uid, time_from, time_to, latest_sequence, error_code, error_message, created_at, started_at, completed_at, updated_at, version FROM tasks WHERE tenant_id = ? AND id = ?`, tenantID, taskID)
	return scanTask(row)
}

func (s *Store) updateTask(ctx context.Context, value task.AnalysisTask, expectedVersion int64) error {
	if err := validateTask(value); err != nil {
		return err
	}
	if expectedVersion < 1 || value.Version != expectedVersion+1 {
		return common.NewError(common.InvalidArgument, "task version update is invalid", false)
	}
	var errorCode, errorMessage any
	if value.Error != nil {
		errorCode, errorMessage = string(value.Error.Code), value.Error.Message
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE tasks SET status = ?, datasource_uid = ?, time_from = ?, time_to = ?, latest_sequence = ?, error_code = ?, error_message = ?, started_at = ?, completed_at = ?, updated_at = ?, version = ? WHERE tenant_id = ? AND id = ? AND version = ?`, value.Status, value.DatasourceUID, storageTimestamp(value.TimeRange.From), storageTimestamp(value.TimeRange.To), value.LatestSequence, errorCode, errorMessage, nullableTimestamp(value.StartedAt), nullableTimestamp(value.CompletedAt), storageTimestamp(value.UpdatedAt), value.Version, value.TenantID, value.ID, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "task")
}

func (s *Store) createToolCall(ctx context.Context, value task.ToolCallRecord) error {
	if err := validateToolCall(value); err != nil {
		return err
	}
	if err := s.ensureTask(ctx, value.TenantID, value.TaskID); err != nil {
		return err
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO tool_calls (id, tenant_id, task_id, tool_name, tool_version, status, input_summary_json, output_summary_json, error_code, error_message, started_at, completed_at, duration_ms, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.TaskID, value.ToolName, value.ToolVersion, value.Status, jsonString(value.InputSummary), nullableJSON(value.OutputSummary), nullableErrorCode(value.Error), nullableErrorMessage(value.Error), storageTimestamp(value.StartedAt), nullableTimestamp(value.CompletedAt), nullableInt64(value.DurationMS), value.Version)
	return mapError(err)
}

func (s *Store) completeToolCall(ctx context.Context, value task.ToolCallRecord, expectedVersion int64) error {
	if err := validateToolCall(value); err != nil {
		return err
	}
	if expectedVersion < 1 || value.Version != expectedVersion+1 || value.Status == task.ToolCallStarted {
		return common.NewError(common.InvalidArgument, "tool call completion is invalid", false)
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE tool_calls SET status = ?, output_summary_json = ?, error_code = ?, error_message = ?, completed_at = ?, duration_ms = ?, version = ? WHERE tenant_id = ? AND id = ? AND version = ?`, value.Status, nullableJSON(value.OutputSummary), nullableErrorCode(value.Error), nullableErrorMessage(value.Error), nullableTimestamp(value.CompletedAt), nullableInt64(value.DurationMS), value.Version, value.TenantID, value.ID, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "tool call")
}

func (s *Store) listToolCallsByTask(ctx context.Context, tenantID, taskID string) ([]task.ToolCallRecord, error) {
	if tenantID == "" || taskID == "" {
		return nil, common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	if err := s.ensureTask(ctx, tenantID, taskID); err != nil {
		return nil, err
	}
	rows, err := s.executor().QueryContext(ctx, `SELECT id, tenant_id, task_id, tool_name, tool_version, status, input_summary_json, output_summary_json, error_code, error_message, started_at, completed_at, duration_ms, version FROM tool_calls WHERE tenant_id = ? AND task_id = ? ORDER BY started_at, id`, tenantID, taskID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	values := make([]task.ToolCallRecord, 0)
	for rows.Next() {
		value, err := scanToolCall(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return values, nil
}

func (s *Store) createChart(ctx context.Context, value chart.ChartDraft) error {
	if err := validateChart(value); err != nil {
		return err
	}
	if err := s.ensureSession(ctx, value.TenantID, value.SessionID); err != nil {
		return err
	}
	if err := s.ensureTaskForSession(ctx, value.TenantID, value.SessionID, value.TaskID); err != nil {
		return err
	}
	queriesJSON, err := json.Marshal(value.Queries)
	if err != nil {
		return mapError(err)
	}
	_, err = s.executor().ExecContext(ctx, `INSERT INTO charts (id, tenant_id, session_id, task_id, title, visualization, unit, queries_json, status, latest_execution_id, created_at, updated_at, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.SessionID, value.TaskID, value.Title, value.Visualization, value.Unit, string(queriesJSON), value.Status, nullableString(value.LatestExecutionID), storageTimestamp(value.CreatedAt), storageTimestamp(value.UpdatedAt), value.Version)
	return mapError(err)
}

func (s *Store) getChart(ctx context.Context, tenantID, chartID string) (chart.ChartDraft, error) {
	if tenantID == "" || chartID == "" {
		return chart.ChartDraft{}, common.NewError(common.InvalidArgument, "tenant and chart are required", false)
	}
	row := s.executor().QueryRowContext(ctx, `SELECT id, tenant_id, session_id, task_id, title, visualization, unit, queries_json, status, latest_execution_id, created_at, updated_at, version FROM charts WHERE tenant_id = ? AND id = ?`, tenantID, chartID)
	return scanChart(row)
}

func (s *Store) updateChart(ctx context.Context, value chart.ChartDraft, expectedVersion int64) error {
	if err := validateChart(value); err != nil {
		return err
	}
	if expectedVersion < 1 || value.Version != expectedVersion+1 {
		return common.NewError(common.InvalidArgument, "chart version update is invalid", false)
	}
	queriesJSON, err := json.Marshal(value.Queries)
	if err != nil {
		return mapError(err)
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE charts SET title = ?, visualization = ?, unit = ?, queries_json = ?, status = ?, latest_execution_id = ?, updated_at = ?, version = ? WHERE tenant_id = ? AND id = ? AND version = ?`, value.Title, value.Visualization, value.Unit, string(queriesJSON), value.Status, nullableString(value.LatestExecutionID), storageTimestamp(value.UpdatedAt), value.Version, value.TenantID, value.ID, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "chart")
}

func (s *Store) listChartsByTask(ctx context.Context, tenantID, taskID string) ([]chart.ChartDraft, error) {
	if tenantID == "" || taskID == "" {
		return nil, common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	if err := s.ensureTask(ctx, tenantID, taskID); err != nil {
		return nil, err
	}
	rows, err := s.executor().QueryContext(ctx, `SELECT id, tenant_id, session_id, task_id, title, visualization, unit, queries_json, status, latest_execution_id, created_at, updated_at, version FROM charts WHERE tenant_id = ? AND task_id = ? ORDER BY created_at, id`, tenantID, taskID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	values := make([]chart.ChartDraft, 0)
	for rows.Next() {
		value, err := scanChart(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return values, nil
}

func (s *Store) createChartExecution(ctx context.Context, value chart.Execution) error {
	if err := validateExecution(value); err != nil {
		return err
	}
	if err := s.ensureChart(ctx, value.TenantID, value.ChartID); err != nil {
		return err
	}
	seriesJSON, err := json.Marshal(value.Series)
	if err != nil {
		return mapError(err)
	}
	warningsJSON, err := json.Marshal(value.Warnings)
	if err != nil {
		return mapError(err)
	}
	_, err = s.executor().ExecContext(ctx, `INSERT INTO chart_executions (id, tenant_id, chart_id, query_ref_id, status, series_count, duration_ms, sample_from, sample_to, series_json, warnings_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.ChartID, value.QueryRefID, value.Status, len(value.Series), value.DurationMS, storageTimestamp(value.SampleRange.From), storageTimestamp(value.SampleRange.To), string(seriesJSON), string(warningsJSON), storageTimestamp(value.CreatedAt))
	return mapError(err)
}

func (s *Store) listChartExecutionsByChart(ctx context.Context, tenantID, chartID string) ([]chart.Execution, error) {
	if tenantID == "" || chartID == "" {
		return nil, common.NewError(common.InvalidArgument, "tenant and chart are required", false)
	}
	if err := s.ensureChart(ctx, tenantID, chartID); err != nil {
		return nil, err
	}
	rows, err := s.executor().QueryContext(ctx, `SELECT id, tenant_id, chart_id, query_ref_id, status, duration_ms, sample_from, sample_to, series_json, warnings_json, created_at FROM chart_executions WHERE tenant_id = ? AND chart_id = ? ORDER BY created_at, id`, tenantID, chartID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	values := make([]chart.Execution, 0)
	for rows.Next() {
		value, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return values, nil
}

func (s *Store) reserve(ctx context.Context, key repositories.IdempotencyKey, requestHash string, expiresAt time.Time) (repositories.IdempotencyRecord, error) {
	if key.TenantID == "" || key.Scope == "" || key.Key == "" || requestHash == "" || expiresAt.IsZero() {
		return repositories.IdempotencyRecord{}, common.NewError(common.InvalidArgument, "idempotency reservation is invalid", false)
	}
	if s.tx == nil {
		var record repositories.IdempotencyRecord
		err := s.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
			var err error
			record, err = tx.Idempotency().Reserve(ctx, key, requestHash, expiresAt)
			return err
		})
		return record, err
	}
	return s.reserveInTransaction(ctx, key, requestHash, expiresAt.UTC())
}

func (s *Store) reserveInTransaction(ctx context.Context, key repositories.IdempotencyKey, requestHash string, expiresAt time.Time) (repositories.IdempotencyRecord, error) {
	now := nowUTC()
	inserted, err := s.executor().ExecContext(ctx, `INSERT OR IGNORE INTO idempotency_keys (tenant_id, scope, key, request_hash, status, resource_id, response_json, created_at, expires_at) VALUES (?, ?, ?, ?, 'reserved', NULL, NULL, ?, ?)`, key.TenantID, key.Scope, key.Key, requestHash, storageTimestamp(now), storageTimestamp(expiresAt))
	if err != nil {
		return repositories.IdempotencyRecord{}, mapError(err)
	}
	if rows, err := inserted.RowsAffected(); err != nil {
		return repositories.IdempotencyRecord{}, mapError(err)
	} else if rows == 1 {
		return repositories.IdempotencyRecord{Key: key, RequestHash: requestHash, Status: "reserved", CreatedAt: now, ExpiresAt: expiresAt}, nil
	}

	record, err := s.getIdempotency(ctx, key)
	if err != nil {
		return repositories.IdempotencyRecord{}, err
	}
	if record.ExpiresAt.Before(now) || record.ExpiresAt.Equal(now) {
		result, err := s.executor().ExecContext(ctx, `UPDATE idempotency_keys SET request_hash = ?, status = 'reserved', resource_id = NULL, response_json = NULL, created_at = ?, expires_at = ? WHERE tenant_id = ? AND scope = ? AND key = ? AND expires_at <= ?`, requestHash, storageTimestamp(now), storageTimestamp(expiresAt), key.TenantID, key.Scope, key.Key, storageTimestamp(now))
		if err != nil {
			return repositories.IdempotencyRecord{}, mapError(err)
		}
		if rows, err := result.RowsAffected(); err != nil {
			return repositories.IdempotencyRecord{}, mapError(err)
		} else if rows == 1 {
			return repositories.IdempotencyRecord{Key: key, RequestHash: requestHash, Status: "reserved", CreatedAt: now, ExpiresAt: expiresAt}, nil
		}
		record, err = s.getIdempotency(ctx, key)
		if err != nil {
			return repositories.IdempotencyRecord{}, err
		}
	}
	if record.RequestHash != requestHash {
		return repositories.IdempotencyRecord{}, common.NewError(common.IdempotencyConflict, "idempotency key was already used with a different request", false)
	}
	return record, nil
}

func (s *Store) getIdempotencyResult(ctx context.Context, key repositories.IdempotencyKey) (repositories.IdempotencyRecord, error) {
	if key.TenantID == "" || key.Scope == "" || key.Key == "" {
		return repositories.IdempotencyRecord{}, common.NewError(common.InvalidArgument, "idempotency key is invalid", false)
	}
	return s.getIdempotency(ctx, key)
}

func (s *Store) completeIdempotency(ctx context.Context, key repositories.IdempotencyKey, resourceID string, responseJSON []byte) error {
	if key.TenantID == "" || key.Scope == "" || key.Key == "" || resourceID == "" || !json.Valid(responseJSON) {
		return common.NewError(common.InvalidArgument, "idempotency completion is invalid", false)
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE idempotency_keys SET status = 'completed', resource_id = ?, response_json = ? WHERE tenant_id = ? AND scope = ? AND key = ?`, resourceID, string(responseJSON), key.TenantID, key.Scope, key.Key)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "idempotency key")
}

func (s *Store) appendEvent(ctx context.Context, draft task.EventDraft) (task.TaskEvent, error) {
	if err := validateEventDraft(draft); err != nil {
		return task.TaskEvent{}, err
	}
	if s.tx == nil {
		var event task.TaskEvent
		err := s.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
			var err error
			event, err = tx.TaskEvents().Append(ctx, draft)
			return err
		})
		return event, err
	}
	return s.appendEventInTransaction(ctx, draft)
}

func (s *Store) appendEventInTransaction(ctx context.Context, draft task.EventDraft) (task.TaskEvent, error) {
	result, err := s.executor().ExecContext(ctx, `UPDATE tasks SET latest_sequence = latest_sequence + 1 WHERE tenant_id = ? AND id = ? AND session_id = ?`, draft.TenantID, draft.TaskID, draft.SessionID)
	if err != nil {
		return task.TaskEvent{}, mapError(err)
	}
	if err := requireUpdated(result, "task"); err != nil {
		return task.TaskEvent{}, err
	}
	var sequence int64
	if err := s.executor().QueryRowContext(ctx, `SELECT latest_sequence FROM tasks WHERE tenant_id = ? AND id = ?`, draft.TenantID, draft.TaskID).Scan(&sequence); err != nil {
		return task.TaskEvent{}, mapError(err)
	}
	if _, err := s.executor().ExecContext(ctx, `INSERT INTO task_events (event_id, tenant_id, task_id, session_id, sequence, type, timestamp, payload_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, draft.EventID, draft.TenantID, draft.TaskID, draft.SessionID, sequence, draft.Type, storageTimestamp(draft.Timestamp), string(draft.Payload)); err != nil {
		return task.TaskEvent{}, mapError(err)
	}
	return task.TaskEvent{EventDraft: cloneEventDraft(draft), Sequence: sequence}, nil
}

func (s *Store) replayEvents(ctx context.Context, tenantID, taskID string, afterSequence int64, limit int) ([]task.TaskEvent, error) {
	if tenantID == "" || taskID == "" || afterSequence < 0 || limit < 1 {
		return nil, common.NewError(common.InvalidArgument, "event replay parameters are invalid", false)
	}
	if limit > maxReplayBatch {
		limit = maxReplayBatch
	}
	if err := s.ensureTask(ctx, tenantID, taskID); err != nil {
		return nil, err
	}
	rows, err := s.executor().QueryContext(ctx, `SELECT event_id, tenant_id, task_id, session_id, sequence, type, timestamp, payload_json FROM task_events WHERE tenant_id = ? AND task_id = ? AND sequence > ? ORDER BY sequence LIMIT ?`, tenantID, taskID, afterSequence, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	events := make([]task.TaskEvent, 0)
	for rows.Next() {
		var event task.TaskEvent
		var timestamp string
		var payload string
		if err := rows.Scan(&event.EventID, &event.TenantID, &event.TaskID, &event.SessionID, &event.Sequence, &event.Type, &timestamp, &payload); err != nil {
			return nil, mapError(err)
		}
		parsed, err := parseTimestamp(timestamp)
		if err != nil {
			return nil, err
		}
		event.Timestamp = parsed
		event.Payload = json.RawMessage(payload)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return events, nil
}

func (s *Store) latestSequence(ctx context.Context, tenantID, taskID string) (int64, error) {
	if tenantID == "" || taskID == "" {
		return 0, common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	var sequence int64
	err := s.executor().QueryRowContext(ctx, `SELECT latest_sequence FROM tasks WHERE tenant_id = ? AND id = ?`, tenantID, taskID).Scan(&sequence)
	if err != nil {
		return 0, mapError(err)
	}
	return sequence, nil
}

func (s *Store) executor() queryer {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

func (s *Store) ensureActive() error {
	if s.db == nil {
		return common.NewError(common.InternalError, "SQLite store is not initialized", true)
	}
	if s.txState == nil {
		return nil
	}
	s.txState.mu.RLock()
	active := s.txState.active
	s.txState.mu.RUnlock()
	if !active {
		return common.NewError(common.InvalidStateTransition, "transaction-scoped store can no longer be used", false)
	}
	return nil
}

func (s *transactionState) deactivate() {
	s.mu.Lock()
	s.active = false
	s.mu.Unlock()
}

func (s *Store) ensureSession(ctx context.Context, tenantID, sessionID string) error {
	var found int
	err := s.executor().QueryRowContext(ctx, `SELECT 1 FROM sessions WHERE tenant_id = ? AND id = ?`, tenantID, sessionID).Scan(&found)
	return mapError(err)
}

func (s *Store) ensureTask(ctx context.Context, tenantID, taskID string) error {
	var found int
	err := s.executor().QueryRowContext(ctx, `SELECT 1 FROM tasks WHERE tenant_id = ? AND id = ?`, tenantID, taskID).Scan(&found)
	return mapError(err)
}

func (s *Store) ensureTaskForSession(ctx context.Context, tenantID, sessionID, taskID string) error {
	var found int
	err := s.executor().QueryRowContext(ctx, `SELECT 1 FROM tasks WHERE tenant_id = ? AND session_id = ? AND id = ?`, tenantID, sessionID, taskID).Scan(&found)
	return mapError(err)
}

func (s *Store) ensureMessage(ctx context.Context, tenantID, sessionID, messageID string) error {
	var found int
	err := s.executor().QueryRowContext(ctx, `SELECT 1 FROM messages WHERE tenant_id = ? AND session_id = ? AND id = ?`, tenantID, sessionID, messageID).Scan(&found)
	return mapError(err)
}

func (s *Store) ensureChart(ctx context.Context, tenantID, chartID string) error {
	var found int
	err := s.executor().QueryRowContext(ctx, `SELECT 1 FROM charts WHERE tenant_id = ? AND id = ?`, tenantID, chartID).Scan(&found)
	return mapError(err)
}

func (s *Store) getIdempotency(ctx context.Context, key repositories.IdempotencyKey) (repositories.IdempotencyRecord, error) {
	var record repositories.IdempotencyRecord
	var createdAt, expiresAt string
	var resourceID, responseJSON sql.NullString
	err := s.executor().QueryRowContext(ctx, `SELECT request_hash, status, resource_id, response_json, created_at, expires_at FROM idempotency_keys WHERE tenant_id = ? AND scope = ? AND key = ?`, key.TenantID, key.Scope, key.Key).Scan(&record.RequestHash, &record.Status, &resourceID, &responseJSON, &createdAt, &expiresAt)
	if err != nil {
		return repositories.IdempotencyRecord{}, mapError(err)
	}
	record.Key = key
	if resourceID.Valid {
		record.ResourceID = resourceID.String
	}
	if responseJSON.Valid {
		record.ResponseJSON = []byte(responseJSON.String)
	}
	var parseErr error
	if record.CreatedAt, parseErr = parseTimestamp(createdAt); parseErr != nil {
		return repositories.IdempotencyRecord{}, parseErr
	}
	if record.ExpiresAt, parseErr = parseTimestamp(expiresAt); parseErr != nil {
		return repositories.IdempotencyRecord{}, parseErr
	}
	return record, nil
}

func scanSession(scanner interface{ Scan(...any) error }) (session.AnalysisSession, error) {
	var value session.AnalysisSession
	var createdAt, updatedAt string
	if err := scanner.Scan(&value.ID, &value.TenantID, &value.Title, &value.Status, &value.CreatedBy, &createdAt, &updatedAt, &value.Version); err != nil {
		return session.AnalysisSession{}, mapError(err)
	}
	var err error
	if value.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return session.AnalysisSession{}, err
	}
	if value.UpdatedAt, err = parseTimestamp(updatedAt); err != nil {
		return session.AnalysisSession{}, err
	}
	return value, nil
}

func scanTask(scanner interface{ Scan(...any) error }) (task.AnalysisTask, error) {
	var value task.AnalysisTask
	var from, to, createdAt, updatedAt string
	var errorCode, errorMessage, startedAt, completedAt sql.NullString
	if err := scanner.Scan(&value.ID, &value.TenantID, &value.SessionID, &value.Status, &value.InputMessageID, &value.DatasourceUID, &from, &to, &value.LatestSequence, &errorCode, &errorMessage, &createdAt, &startedAt, &completedAt, &updatedAt, &value.Version); err != nil {
		return task.AnalysisTask{}, mapError(err)
	}
	var err error
	if value.TimeRange.From, err = parseTimestamp(from); err != nil {
		return task.AnalysisTask{}, err
	}
	if value.TimeRange.To, err = parseTimestamp(to); err != nil {
		return task.AnalysisTask{}, err
	}
	if value.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return task.AnalysisTask{}, err
	}
	if value.UpdatedAt, err = parseTimestamp(updatedAt); err != nil {
		return task.AnalysisTask{}, err
	}
	if value.StartedAt, err = parseNullableTimestamp(startedAt); err != nil {
		return task.AnalysisTask{}, err
	}
	if value.CompletedAt, err = parseNullableTimestamp(completedAt); err != nil {
		return task.AnalysisTask{}, err
	}
	if errorCode.Valid {
		value.Error = common.NewError(common.ErrorCode(errorCode.String), errorMessage.String, false)
	}
	return value, nil
}

func scanToolCall(scanner interface{ Scan(...any) error }) (task.ToolCallRecord, error) {
	var value task.ToolCallRecord
	var inputJSON string
	var outputJSON, errorCode, errorMessage, completedAt sql.NullString
	var duration sql.NullInt64
	var startedAt string
	if err := scanner.Scan(&value.ID, &value.TenantID, &value.TaskID, &value.ToolName, &value.ToolVersion, &value.Status, &inputJSON, &outputJSON, &errorCode, &errorMessage, &startedAt, &completedAt, &duration, &value.Version); err != nil {
		return task.ToolCallRecord{}, mapError(err)
	}
	value.InputSummary = json.RawMessage(inputJSON)
	if outputJSON.Valid {
		value.OutputSummary = json.RawMessage(outputJSON.String)
	}
	var err error
	if value.StartedAt, err = parseTimestamp(startedAt); err != nil {
		return task.ToolCallRecord{}, err
	}
	if value.CompletedAt, err = parseNullableTimestamp(completedAt); err != nil {
		return task.ToolCallRecord{}, err
	}
	if duration.Valid {
		durationValue := duration.Int64
		value.DurationMS = &durationValue
	}
	if errorCode.Valid {
		value.Error = common.NewError(common.ErrorCode(errorCode.String), errorMessage.String, false)
	}
	return value, nil
}

func scanChart(scanner interface{ Scan(...any) error }) (chart.ChartDraft, error) {
	var value chart.ChartDraft
	var queriesJSON, createdAt, updatedAt string
	var latestExecutionID sql.NullString
	if err := scanner.Scan(&value.ID, &value.TenantID, &value.SessionID, &value.TaskID, &value.Title, &value.Visualization, &value.Unit, &queriesJSON, &value.Status, &latestExecutionID, &createdAt, &updatedAt, &value.Version); err != nil {
		return chart.ChartDraft{}, mapError(err)
	}
	if err := json.Unmarshal([]byte(queriesJSON), &value.Queries); err != nil {
		return chart.ChartDraft{}, mapError(err)
	}
	var err error
	if value.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return chart.ChartDraft{}, err
	}
	if value.UpdatedAt, err = parseTimestamp(updatedAt); err != nil {
		return chart.ChartDraft{}, err
	}
	if latestExecutionID.Valid {
		id := latestExecutionID.String
		value.LatestExecutionID = &id
	}
	return value, nil
}

func scanExecution(scanner interface{ Scan(...any) error }) (chart.Execution, error) {
	var value chart.Execution
	var from, to, seriesJSON, warningsJSON, createdAt string
	if err := scanner.Scan(&value.ID, &value.TenantID, &value.ChartID, &value.QueryRefID, &value.Status, &value.DurationMS, &from, &to, &seriesJSON, &warningsJSON, &createdAt); err != nil {
		return chart.Execution{}, mapError(err)
	}
	if err := json.Unmarshal([]byte(seriesJSON), &value.Series); err != nil {
		return chart.Execution{}, mapError(err)
	}
	if err := json.Unmarshal([]byte(warningsJSON), &value.Warnings); err != nil {
		return chart.Execution{}, mapError(err)
	}
	var err error
	if value.SampleRange.From, err = parseTimestamp(from); err != nil {
		return chart.Execution{}, err
	}
	if value.SampleRange.To, err = parseTimestamp(to); err != nil {
		return chart.Execution{}, err
	}
	if value.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return chart.Execution{}, err
	}
	return value, nil
}

func validateTask(value task.AnalysisTask) error {
	if value.ID == "" || value.TenantID == "" || value.SessionID == "" || value.InputMessageID == "" || value.DatasourceUID == "" || !value.TimeRange.From.Before(value.TimeRange.To) || value.LatestSequence < 0 || value.Version < 1 || !isTaskStatus(value.Status) {
		return common.NewError(common.InvalidArgument, "task is invalid", false)
	}
	return nil
}

func validateToolCall(value task.ToolCallRecord) error {
	if value.ID == "" || value.TenantID == "" || value.TaskID == "" || value.ToolName == "" || value.ToolVersion != "v1" || value.Version < 1 || !isToolCallStatus(value.Status) || !json.Valid(value.InputSummary) {
		return common.NewError(common.InvalidArgument, "tool call is invalid", false)
	}
	if len(value.OutputSummary) > 0 && !json.Valid(value.OutputSummary) {
		return common.NewError(common.InvalidArgument, "tool call output summary is invalid", false)
	}
	if value.Status == task.ToolCallStarted && (value.CompletedAt != nil || value.DurationMS != nil || value.Error != nil) {
		return common.NewError(common.InvalidArgument, "started tool call cannot have a completion", false)
	}
	if value.Status != task.ToolCallStarted && (value.CompletedAt == nil || value.DurationMS == nil) {
		return common.NewError(common.InvalidArgument, "completed tool call requires duration and completion time", false)
	}
	return nil
}

func validateChart(value chart.ChartDraft) error {
	if value.ID == "" || value.TenantID == "" || value.SessionID == "" || value.TaskID == "" || value.Title == "" || value.Unit == "" || value.Visualization != "timeseries" || value.Version < 1 || (value.Status != chart.StatusProposed && value.Status != chart.StatusReady) || len(value.Queries) == 0 {
		return common.NewError(common.InvalidArgument, "chart is invalid", false)
	}
	for _, query := range value.Queries {
		if query.RefID == "" || query.Expression == "" || query.Legend == "" || query.DatasourceUID == "" || !query.TimeRange.From.Before(query.TimeRange.To) {
			return common.NewError(common.InvalidArgument, "chart query is invalid", false)
		}
	}
	return nil
}

func validateExecution(value chart.Execution) error {
	if value.ID == "" || value.TenantID == "" || value.ChartID == "" || value.QueryRefID == "" || value.DurationMS < 0 || !value.SampleRange.From.Before(value.SampleRange.To) || (value.Status != chart.ExecutionSuccess && value.Status != chart.ExecutionFailed) {
		return common.NewError(common.InvalidArgument, "chart execution is invalid", false)
	}
	return nil
}

func validateEventDraft(value task.EventDraft) error {
	if value.EventID == "" || value.TenantID == "" || value.TaskID == "" || value.SessionID == "" || value.Timestamp.IsZero() || !isEventType(value.Type) || !isJSONObject(value.Payload) {
		return common.NewError(common.InvalidArgument, "task event is invalid", false)
	}
	return nil
}

func isTaskStatus(status task.Status) bool {
	return status == task.StatusCreated || status == task.StatusPlanning || status == task.StatusRunningTools || status == task.StatusValidating || status == task.StatusCompleted || status == task.StatusFailed
}

func isToolCallStatus(status task.ToolCallStatus) bool {
	return status == task.ToolCallStarted || status == task.ToolCallCompleted || status == task.ToolCallFailed
}

func isEventType(eventType task.EventType) bool {
	switch eventType {
	case task.EventTaskCreated, task.EventTaskStatusChanged, task.EventAssistantMessageStarted, task.EventAssistantMessageDelta, task.EventAssistantMessageDone, task.EventToolStarted, task.EventToolCompleted, task.EventToolFailed, task.EventMetricCandidatesCreated, task.EventChartCreated, task.EventChartExecutionDone, task.EventTaskCompleted, task.EventTaskFailed:
		return true
	default:
		return false
	}
}

func isJSONObject(value []byte) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(value, &object) == nil && object != nil
}

func requireUpdated(result sql.Result, resource string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return mapError(err)
	}
	if rows == 0 {
		return common.NewError(common.ResourceConflict, fmt.Sprintf("%s was changed by another request or does not exist", resource), false)
	}
	return nil
}

func storageTimestamp(value time.Time) string { return value.UTC().Format(storageTimeLayout) }

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(storageTimeLayout, value)
	if err != nil {
		return time.Time{}, common.NewError(common.InternalError, "SQLite contains an invalid timestamp", false)
	}
	return parsed.UTC(), nil
}

func nullableTimestamp(value *time.Time) any {
	if value == nil {
		return nil
	}
	return storageTimestamp(*value)
}

func parseNullableTimestamp(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseTimestamp(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func jsonString(value json.RawMessage) string { return string(value) }

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

func nullableErrorCode(value *common.DomainError) any {
	if value == nil {
		return nil
	}
	return string(value.Code)
}

func nullableErrorMessage(value *common.DomainError) any {
	if value == nil {
		return nil
	}
	return value.Message
}

func cloneEventDraft(value task.EventDraft) task.EventDraft {
	value.Payload = append(json.RawMessage(nil), value.Payload...)
	value.Timestamp = value.Timestamp.UTC()
	return value
}
