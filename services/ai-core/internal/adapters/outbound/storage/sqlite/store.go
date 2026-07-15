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
	"mini-torchbearing.local/services/ai-core/internal/domain/incident"
	"mini-torchbearing.local/services/ai-core/internal/domain/remediation"
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
	writeMu *sync.Mutex
}

func (s *Store) Sessions() repositories.SessionRepository { return sessionRepository{store: s} }
func (s *Store) Messages() repositories.MessageRepository { return messageRepository{store: s} }
func (s *Store) Tasks() repositories.TaskRepository       { return taskRepository{store: s} }
func (s *Store) TaskCheckpoints() repositories.TaskCheckpointRepository {
	return taskCheckpointRepository{store: s}
}
func (s *Store) AlertEvents() repositories.AlertEventRepository {
	return alertEventRepository{store: s}
}
func (s *Store) RemediationIntents() repositories.RemediationIntentRepository {
	return remediationIntentRepository{store: s}
}
func (s *Store) Approvals() repositories.ApprovalRepository {
	return approvalRepository{store: s}
}
func (s *Store) RemediationExecutions() repositories.RemediationExecutionRepository {
	return remediationExecutionRepository{store: s}
}
func (s *Store) AuditRecords() repositories.AuditRecordRepository {
	return auditRecordRepository{store: s}
}
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
func (r sessionRepository) GetOwned(ctx context.Context, tenantID, userID, sessionID string) (session.AnalysisSession, error) {
	return r.store.getOwnedSession(ctx, tenantID, userID, sessionID)
}
func (r sessionRepository) ListPageByOwner(ctx context.Context, tenantID, userID string, page repositories.SessionListRequest) (repositories.SessionListPage, error) {
	return r.store.listSessionPageByOwner(ctx, tenantID, userID, page)
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
func (r messageRepository) ListPageBySession(ctx context.Context, tenantID, sessionID string, page repositories.PageRequest) (repositories.Page[session.Message], error) {
	return r.store.listMessagePageBySession(ctx, tenantID, sessionID, page)
}

type taskRepository struct{ store *Store }

func (r taskRepository) Create(ctx context.Context, value task.AnalysisTask) error {
	return r.store.createTask(ctx, value)
}
func (r taskRepository) Get(ctx context.Context, tenantID, taskID string) (task.AnalysisTask, error) {
	return r.store.getTask(ctx, tenantID, taskID)
}
func (r taskRepository) ListNonTerminal(ctx context.Context) ([]task.AnalysisTask, error) {
	return r.store.listNonTerminalTasks(ctx)
}
func (r taskRepository) ListPageBySession(ctx context.Context, tenantID, sessionID string, page repositories.PageRequest) (repositories.Page[task.AnalysisTask], error) {
	return r.store.listTaskPageBySession(ctx, tenantID, sessionID, page)
}
func (r taskRepository) Update(ctx context.Context, value task.AnalysisTask, expectedVersion int64) error {
	return r.store.updateTask(ctx, value, expectedVersion)
}

type taskCheckpointRepository struct{ store *Store }

func (r taskCheckpointRepository) Create(ctx context.Context, value task.Checkpoint) error {
	return r.store.createTaskCheckpoint(ctx, value)
}

type alertEventRepository struct{ store *Store }

func (r alertEventRepository) Create(ctx context.Context, value incident.AlertEvent) error {
	return r.store.createAlertEvent(ctx, value)
}
func (r alertEventRepository) GetByKey(ctx context.Context, key incident.AlertKey) (incident.AlertEvent, error) {
	return r.store.getAlertEventByKey(ctx, key)
}

type remediationIntentRepository struct{ store *Store }

func (r remediationIntentRepository) Create(ctx context.Context, value remediation.IntentRecord) error {
	return r.store.createRemediationIntent(ctx, value)
}
func (r remediationIntentRepository) GetByTask(ctx context.Context, tenantID, taskID string) (remediation.IntentRecord, error) {
	return r.store.getRemediationIntentByTask(ctx, tenantID, taskID)
}

type approvalRepository struct{ store *Store }

func (r approvalRepository) Create(ctx context.Context, value remediation.Approval) error {
	return r.store.createApproval(ctx, value)
}
func (r approvalRepository) GetByTask(ctx context.Context, tenantID, taskID string) (remediation.Approval, error) {
	return r.store.getApprovalByTask(ctx, tenantID, taskID)
}
func (r approvalRepository) Update(ctx context.Context, value remediation.Approval, expectedVersion int64) error {
	return r.store.updateApproval(ctx, value, expectedVersion)
}

type remediationExecutionRepository struct{ store *Store }

func (r remediationExecutionRepository) Create(ctx context.Context, value remediation.Execution) error {
	return r.store.createRemediationExecution(ctx, value)
}
func (r remediationExecutionRepository) GetByTask(ctx context.Context, tenantID, taskID string) (remediation.Execution, error) {
	return r.store.getRemediationExecutionByTask(ctx, tenantID, taskID)
}
func (r remediationExecutionRepository) GetByOperation(ctx context.Context, tenantID, operationID string) (remediation.Execution, error) {
	return r.store.getRemediationExecutionByOperation(ctx, tenantID, operationID)
}
func (r remediationExecutionRepository) Update(ctx context.Context, value remediation.Execution, expectedVersion int64) error {
	return r.store.updateRemediationExecution(ctx, value, expectedVersion)
}

type auditRecordRepository struct{ store *Store }

func (r auditRecordRepository) Create(ctx context.Context, value remediation.AuditRecord) error {
	return r.store.createAuditRecord(ctx, value)
}
func (r auditRecordRepository) ListByTask(ctx context.Context, tenantID, taskID string) ([]remediation.AuditRecord, error) {
	return r.store.listAuditRecordsByTask(ctx, tenantID, taskID)
}
func (r taskCheckpointRepository) Get(ctx context.Context, tenantID, taskID string) (task.Checkpoint, error) {
	return r.store.getTaskCheckpoint(ctx, tenantID, taskID)
}
func (r taskCheckpointRepository) Update(ctx context.Context, value task.Checkpoint, expectedVersion int64) error {
	return r.store.updateTaskCheckpoint(ctx, value, expectedVersion)
}
func (r taskCheckpointRepository) Delete(ctx context.Context, tenantID, taskID string, expectedVersion int64) error {
	return r.store.deleteTaskCheckpoint(ctx, tenantID, taskID, expectedVersion)
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
func (r eventStore) ReplayTo(ctx context.Context, tenantID, taskID string, afterSequence, targetSequence int64, limit int) ([]task.TaskEvent, error) {
	return r.store.replayEventsTo(ctx, tenantID, taskID, afterSequence, targetSequence, limit)
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
	if s.writeMu != nil {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapError(err)
	}
	state := &transactionState{active: true}
	txStore := &Store{db: s.db, tx: tx, txState: state, writeMu: s.writeMu}
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
	if value.ID == "" || value.TenantID == "" || value.OrgID == "" || value.CreatedBy == "" || strings.TrimSpace(value.Title) == "" || value.Status != session.StatusActive || (value.Kind != session.KindPrivate && value.Kind != session.KindOrgIncident) || value.Version < 1 {
		return common.NewError(common.InvalidArgument, "session is invalid", false)
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO sessions (id, tenant_id, org_id, kind, title, status, created_by, created_at, updated_at, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.OrgID, value.Kind, value.Title, value.Status, value.CreatedBy, storageTimestamp(value.CreatedAt), storageTimestamp(value.UpdatedAt), value.Version)
	return mapError(err)
}

func (s *Store) getSession(ctx context.Context, tenantID, sessionID string) (session.AnalysisSession, error) {
	if tenantID == "" || sessionID == "" {
		return session.AnalysisSession{}, common.NewError(common.InvalidArgument, "tenant and session are required", false)
	}
	row := s.executor().QueryRowContext(ctx, `SELECT id, tenant_id, org_id, kind, title, status, created_by, created_at, updated_at, version FROM sessions WHERE tenant_id = ? AND id = ?`, tenantID, sessionID)
	return scanSession(row)
}

func (s *Store) getOwnedSession(ctx context.Context, tenantID, userID, sessionID string) (session.AnalysisSession, error) {
	if tenantID == "" || userID == "" || sessionID == "" {
		return session.AnalysisSession{}, common.NewError(common.InvalidArgument, "tenant, user and session are required", false)
	}
	row := s.executor().QueryRowContext(ctx, `SELECT id, tenant_id, org_id, kind, title, status, created_by, created_at, updated_at, version FROM sessions WHERE tenant_id = ? AND kind = 'private' AND created_by = ? AND id = ?`, tenantID, userID, sessionID)
	return scanSession(row)
}

func (s *Store) listSessionPageByOwner(ctx context.Context, tenantID, userID string, page repositories.SessionListRequest) (repositories.SessionListPage, error) {
	if tenantID == "" || userID == "" || page.Limit < 1 || page.Limit > 50 || (page.BeforeUpdatedAt == nil && page.BeforeID != "") || (page.BeforeUpdatedAt != nil && page.BeforeID == "") {
		return repositories.SessionListPage{}, common.NewError(common.InvalidArgument, "session page request is invalid", false)
	}
	query := `SELECT s.id, s.tenant_id, s.org_id, s.kind, s.title, s.status, s.created_by, s.created_at, s.updated_at, s.version FROM sessions s WHERE s.tenant_id = ? AND s.kind = 'private' AND s.created_by = ? AND EXISTS (SELECT 1 FROM tasks t WHERE t.tenant_id = s.tenant_id AND t.session_id = s.id)`
	args := []any{tenantID, userID}
	if page.BeforeUpdatedAt != nil {
		query += ` AND (s.updated_at < ? OR (s.updated_at = ? AND s.id < ?))`
		cursor := storageTimestamp(*page.BeforeUpdatedAt)
		args = append(args, cursor, cursor, page.BeforeID)
	}
	query += ` ORDER BY s.updated_at DESC, s.id DESC LIMIT ?`
	args = append(args, page.Limit+1)
	rows, err := s.executor().QueryContext(ctx, query, args...)
	if err != nil {
		return repositories.SessionListPage{}, mapError(err)
	}
	defer rows.Close()
	items := make([]session.AnalysisSession, 0, page.Limit+1)
	for rows.Next() {
		item, err := scanSession(rows)
		if err != nil {
			return repositories.SessionListPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return repositories.SessionListPage{}, mapError(err)
	}
	result := repositories.SessionListPage{Items: items}
	if len(result.Items) > page.Limit {
		result.Items = result.Items[:page.Limit]
		last := result.Items[len(result.Items)-1]
		result.NextAfter = &repositories.SessionListCursor{UpdatedAt: last.UpdatedAt, ID: last.ID}
	}
	return result, nil
}

func (s *Store) updateSession(ctx context.Context, value session.AnalysisSession, expectedVersion int64) error {
	if value.ID == "" || value.TenantID == "" || value.OrgID == "" || expectedVersion < 1 || value.Version != expectedVersion+1 || value.Status != session.StatusActive || (value.Kind != session.KindPrivate && value.Kind != session.KindOrgIncident) {
		return common.NewError(common.InvalidArgument, "session update is invalid", false)
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE sessions SET title = ?, status = ?, updated_at = ?, version = ? WHERE tenant_id = ? AND id = ? AND version = ?`, value.Title, value.Status, storageTimestamp(value.UpdatedAt), value.Version, value.TenantID, value.ID, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "session")
}

func (s *Store) appendMessage(ctx context.Context, value session.Message) error {
	if value.ID == "" || value.TenantID == "" || value.SessionID == "" || value.TaskID == "" || strings.TrimSpace(value.Content) == "" || (value.Role != session.RoleUser && value.Role != session.RoleAssistant && value.Role != session.RoleTrigger) {
		return common.NewError(common.InvalidArgument, "message is invalid", false)
	}
	if err := s.ensureSession(ctx, value.TenantID, value.SessionID); err != nil {
		return err
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO messages (id, tenant_id, session_id, task_id, role, content, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.SessionID, value.TaskID, value.Role, value.Content, storageTimestamp(value.CreatedAt))
	return mapError(err)
}

func (s *Store) listMessagesBySession(ctx context.Context, tenantID, sessionID string) ([]session.Message, error) {
	if tenantID == "" || sessionID == "" {
		return nil, common.NewError(common.InvalidArgument, "tenant and session are required", false)
	}
	if err := s.ensureSession(ctx, tenantID, sessionID); err != nil {
		return nil, err
	}
	rows, err := s.executor().QueryContext(ctx, `SELECT id, tenant_id, session_id, task_id, role, content, created_at FROM messages WHERE tenant_id = ? AND session_id = ? ORDER BY created_at, id`, tenantID, sessionID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	messages := make([]session.Message, 0)
	for rows.Next() {
		var item session.Message
		var createdAt string
		if err := rows.Scan(&item.ID, &item.TenantID, &item.SessionID, &item.TaskID, &item.Role, &item.Content, &createdAt); err != nil {
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

func (s *Store) listMessagePageBySession(ctx context.Context, tenantID, sessionID string, page repositories.PageRequest) (repositories.Page[session.Message], error) {
	limit, err := validatePageRequest(page, 100)
	if err != nil {
		return repositories.Page[session.Message]{}, err
	}
	if err := s.ensureSession(ctx, tenantID, sessionID); err != nil {
		return repositories.Page[session.Message]{}, err
	}
	query := `SELECT id, tenant_id, session_id, task_id, role, content, created_at FROM messages WHERE tenant_id = ? AND session_id = ?`
	args := []any{tenantID, sessionID}
	if page.CreatedAt != nil {
		query += ` AND (created_at < ? OR (created_at = ? AND id < ?))`
		cursor := storageTimestamp(*page.CreatedAt)
		args = append(args, cursor, cursor, page.ID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.executor().QueryContext(ctx, query, args...)
	if err != nil {
		return repositories.Page[session.Message]{}, mapError(err)
	}
	defer rows.Close()
	items := make([]session.Message, 0, limit+1)
	for rows.Next() {
		var item session.Message
		var createdAt string
		if err := rows.Scan(&item.ID, &item.TenantID, &item.SessionID, &item.TaskID, &item.Role, &item.Content, &createdAt); err != nil {
			return repositories.Page[session.Message]{}, mapError(err)
		}
		if item.CreatedAt, err = parseTimestamp(createdAt); err != nil {
			return repositories.Page[session.Message]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return repositories.Page[session.Message]{}, mapError(err)
	}
	return pageMessages(items, limit), nil
}

func (s *Store) createTask(ctx context.Context, value task.AnalysisTask) error {
	if err := validateTask(value); err != nil {
		return err
	}
	if err := s.ensureSession(ctx, value.TenantID, value.SessionID); err != nil {
		return err
	}
	var errorCode, errorMessage any
	if value.Error != nil {
		errorCode, errorMessage = string(value.Error.Code), value.Error.Message
	}
	datasource, from, to, views, step, window, incident, err := taskPlanStorage(value)
	if err != nil {
		return err
	}
	_, err = s.executor().ExecContext(ctx, `INSERT INTO tasks (id, tenant_id, kind, session_id, status, input_message_id, datasource_uid, time_from, time_to, views_json, step_seconds, cpu_rate_window_seconds, incident_plan_json, latest_sequence, error_code, error_message, created_at, started_at, completed_at, updated_at, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.Kind, value.SessionID, value.Status, value.InputMessageID, datasource, from, to, views, step, window, incident, value.LatestSequence, errorCode, errorMessage, storageTimestamp(value.CreatedAt), nullableTimestamp(value.StartedAt), nullableTimestamp(value.CompletedAt), storageTimestamp(value.UpdatedAt), value.Version)
	return mapError(err)
}

func (s *Store) getTask(ctx context.Context, tenantID, taskID string) (task.AnalysisTask, error) {
	if tenantID == "" || taskID == "" {
		return task.AnalysisTask{}, common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	row := s.executor().QueryRowContext(ctx, `SELECT id, tenant_id, kind, session_id, status, input_message_id, datasource_uid, time_from, time_to, views_json, step_seconds, cpu_rate_window_seconds, incident_plan_json, latest_sequence, error_code, error_message, created_at, started_at, completed_at, updated_at, version FROM tasks WHERE tenant_id = ? AND id = ?`, tenantID, taskID)
	return scanTask(row)
}

func (s *Store) listNonTerminalTasks(ctx context.Context) ([]task.AnalysisTask, error) {
	rows, err := s.executor().QueryContext(ctx, `SELECT id, tenant_id, kind, session_id, status, input_message_id, datasource_uid, time_from, time_to, views_json, step_seconds, cpu_rate_window_seconds, incident_plan_json, latest_sequence, error_code, error_message, created_at, started_at, completed_at, updated_at, version FROM tasks WHERE status NOT IN (?, ?, ?) ORDER BY created_at, id`, task.StatusCompleted, task.StatusFailed, task.StatusCancelled)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	values := make([]task.AnalysisTask, 0)
	for rows.Next() {
		value, err := scanTask(rows)
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

func (s *Store) listTaskPageBySession(ctx context.Context, tenantID, sessionID string, page repositories.PageRequest) (repositories.Page[task.AnalysisTask], error) {
	limit, err := validatePageRequest(page, 50)
	if err != nil {
		return repositories.Page[task.AnalysisTask]{}, err
	}
	if err := s.ensureSession(ctx, tenantID, sessionID); err != nil {
		return repositories.Page[task.AnalysisTask]{}, err
	}
	query := `SELECT id, tenant_id, kind, session_id, status, input_message_id, datasource_uid, time_from, time_to, views_json, step_seconds, cpu_rate_window_seconds, incident_plan_json, latest_sequence, error_code, error_message, created_at, started_at, completed_at, updated_at, version FROM tasks WHERE tenant_id = ? AND session_id = ?`
	args := []any{tenantID, sessionID}
	if page.CreatedAt != nil {
		query += ` AND (created_at < ? OR (created_at = ? AND id < ?))`
		cursor := storageTimestamp(*page.CreatedAt)
		args = append(args, cursor, cursor, page.ID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.executor().QueryContext(ctx, query, args...)
	if err != nil {
		return repositories.Page[task.AnalysisTask]{}, mapError(err)
	}
	defer rows.Close()
	items := make([]task.AnalysisTask, 0, limit+1)
	for rows.Next() {
		item, err := scanTask(rows)
		if err != nil {
			return repositories.Page[task.AnalysisTask]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return repositories.Page[task.AnalysisTask]{}, mapError(err)
	}
	return pageTasks(items, limit), nil
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
	datasource, from, to, views, step, window, incident, err := taskPlanStorage(value)
	if err != nil {
		return err
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE tasks SET status = ?, datasource_uid = ?, time_from = ?, time_to = ?, views_json = ?, step_seconds = ?, cpu_rate_window_seconds = ?, incident_plan_json = ?, latest_sequence = ?, error_code = ?, error_message = ?, started_at = ?, completed_at = ?, updated_at = ?, version = ? WHERE tenant_id = ? AND id = ? AND kind = ? AND version = ?`, value.Status, datasource, from, to, views, step, window, incident, value.LatestSequence, errorCode, errorMessage, nullableTimestamp(value.StartedAt), nullableTimestamp(value.CompletedAt), storageTimestamp(value.UpdatedAt), value.Version, value.TenantID, value.ID, value.Kind, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "task")
}

func (s *Store) createTaskCheckpoint(ctx context.Context, value task.Checkpoint) error {
	if err := validateTaskCheckpoint(value); err != nil {
		return err
	}
	if err := s.ensureIncidentTask(ctx, value.TenantID, value.TaskID); err != nil {
		return err
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO task_checkpoints (task_id, tenant_id, phase, opaque_value, updated_at, version) VALUES (?, ?, ?, ?, ?, ?)`, value.TaskID, value.TenantID, value.Phase, value.OpaqueValue, storageTimestamp(value.UpdatedAt), value.Version)
	return mapError(err)
}

func (s *Store) getTaskCheckpoint(ctx context.Context, tenantID, taskID string) (task.Checkpoint, error) {
	if tenantID == "" || taskID == "" {
		return task.Checkpoint{}, common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	var value task.Checkpoint
	var updatedAt string
	err := s.executor().QueryRowContext(ctx, `SELECT task_id, tenant_id, phase, opaque_value, updated_at, version FROM task_checkpoints WHERE tenant_id = ? AND task_id = ?`, tenantID, taskID).Scan(&value.TaskID, &value.TenantID, &value.Phase, &value.OpaqueValue, &updatedAt, &value.Version)
	if err != nil {
		return task.Checkpoint{}, mapError(err)
	}
	if value.UpdatedAt, err = parseTimestamp(updatedAt); err != nil {
		return task.Checkpoint{}, err
	}
	return value, nil
}

func (s *Store) updateTaskCheckpoint(ctx context.Context, value task.Checkpoint, expectedVersion int64) error {
	if err := validateTaskCheckpoint(value); err != nil {
		return err
	}
	if expectedVersion < 1 || value.Version != expectedVersion+1 {
		return common.NewError(common.InvalidArgument, "task checkpoint version update is invalid", false)
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE task_checkpoints SET phase = ?, opaque_value = ?, updated_at = ?, version = ? WHERE tenant_id = ? AND task_id = ? AND version = ?`, value.Phase, value.OpaqueValue, storageTimestamp(value.UpdatedAt), value.Version, value.TenantID, value.TaskID, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "task checkpoint")
}

func (s *Store) deleteTaskCheckpoint(ctx context.Context, tenantID, taskID string, expectedVersion int64) error {
	if tenantID == "" || taskID == "" || expectedVersion < 1 {
		return common.NewError(common.InvalidArgument, "task checkpoint delete is invalid", false)
	}
	result, err := s.executor().ExecContext(ctx, `DELETE FROM task_checkpoints WHERE tenant_id = ? AND task_id = ? AND version = ?`, tenantID, taskID, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "task checkpoint")
}

func (s *Store) createAlertEvent(ctx context.Context, value incident.AlertEvent) error {
	if value.ID == "" || value.Key.TenantID == "" || value.Key.OrgID == "" || value.Key.SourceID == "" || value.Key.Fingerprint == "" || value.Key.StartsAt.IsZero() || (value.Key.Status != incident.AlertFiring && value.Key.Status != incident.AlertResolved) || value.ServiceRef == "" || value.AlertName == "" || len(value.Labels) == 0 || len(value.Labels) > 24 || value.ReceivedAt.IsZero() || (value.Key.Status == incident.AlertFiring && value.TaskID == "") {
		return common.NewError(common.InvalidArgument, "alert event is invalid", false)
	}
	labels, err := json.Marshal(value.Labels)
	if err != nil {
		return mapError(err)
	}
	var taskID any
	if value.TaskID != "" {
		taskID = value.TaskID
	}
	_, err = s.executor().ExecContext(ctx, `INSERT INTO alert_events (id, tenant_id, org_id, source_id, fingerprint, starts_at, status, service_ref, alert_name, labels_json, task_id, received_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.Key.TenantID, value.Key.OrgID, value.Key.SourceID, value.Key.Fingerprint, storageTimestamp(value.Key.StartsAt), value.Key.Status, value.ServiceRef, value.AlertName, string(labels), taskID, storageTimestamp(value.ReceivedAt))
	return mapError(err)
}

func (s *Store) getAlertEventByKey(ctx context.Context, key incident.AlertKey) (incident.AlertEvent, error) {
	if key.TenantID == "" || key.OrgID == "" || key.SourceID == "" || key.Fingerprint == "" || key.StartsAt.IsZero() || (key.Status != incident.AlertFiring && key.Status != incident.AlertResolved) {
		return incident.AlertEvent{}, common.NewError(common.InvalidArgument, "alert key is invalid", false)
	}
	var value incident.AlertEvent
	var startsAt, labelsJSON, receivedAt string
	var taskID sql.NullString
	err := s.executor().QueryRowContext(ctx, `SELECT id, tenant_id, org_id, source_id, fingerprint, starts_at, status, service_ref, alert_name, labels_json, task_id, received_at FROM alert_events WHERE tenant_id = ? AND org_id = ? AND source_id = ? AND fingerprint = ? AND starts_at = ? AND status = ?`, key.TenantID, key.OrgID, key.SourceID, key.Fingerprint, storageTimestamp(key.StartsAt), key.Status).Scan(&value.ID, &value.Key.TenantID, &value.Key.OrgID, &value.Key.SourceID, &value.Key.Fingerprint, &startsAt, &value.Key.Status, &value.ServiceRef, &value.AlertName, &labelsJSON, &taskID, &receivedAt)
	if err != nil {
		return incident.AlertEvent{}, mapError(err)
	}
	if value.Key.StartsAt, err = parseTimestamp(startsAt); err != nil {
		return incident.AlertEvent{}, err
	}
	if value.ReceivedAt, err = parseTimestamp(receivedAt); err != nil {
		return incident.AlertEvent{}, err
	}
	if err := json.Unmarshal([]byte(labelsJSON), &value.Labels); err != nil {
		return incident.AlertEvent{}, common.NewError(common.InternalError, "stored alert labels are invalid", false)
	}
	if taskID.Valid {
		value.TaskID = taskID.String
	}
	return value, nil
}

func (s *Store) createRemediationIntent(ctx context.Context, value remediation.IntentRecord) error {
	if value.TenantID == "" || value.OrgID == "" || value.TaskID == "" || !value.Intent.Valid(value.Intent.ServiceRef) {
		return common.NewError(common.InvalidArgument, "remediation intent record is invalid", false)
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO remediation_intents (id, tenant_id, org_id, task_id, digest, capability_id, service_ref, instance_epoch, expected_version, before_concurrency, after_concurrency, risk, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.Intent.ID, value.TenantID, value.OrgID, value.TaskID, value.Intent.Digest, value.Intent.CapabilityID, value.Intent.ServiceRef, value.Intent.InstanceEpoch, value.Intent.ExpectedVersion, value.Intent.BeforeConcurrency, value.Intent.AfterConcurrency, value.Intent.Risk, storageTimestamp(value.Intent.CreatedAt))
	return mapError(err)
}

func (s *Store) getRemediationIntentByTask(ctx context.Context, tenantID, taskID string) (remediation.IntentRecord, error) {
	if tenantID == "" || taskID == "" {
		return remediation.IntentRecord{}, common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	var value remediation.IntentRecord
	var createdAt string
	err := s.executor().QueryRowContext(ctx, `SELECT tenant_id, org_id, task_id, id, digest, capability_id, service_ref, instance_epoch, expected_version, before_concurrency, after_concurrency, risk, created_at FROM remediation_intents WHERE tenant_id = ? AND task_id = ?`, tenantID, taskID).Scan(&value.TenantID, &value.OrgID, &value.TaskID, &value.Intent.ID, &value.Intent.Digest, &value.Intent.CapabilityID, &value.Intent.ServiceRef, &value.Intent.InstanceEpoch, &value.Intent.ExpectedVersion, &value.Intent.BeforeConcurrency, &value.Intent.AfterConcurrency, &value.Intent.Risk, &createdAt)
	if err != nil {
		return remediation.IntentRecord{}, mapError(err)
	}
	if value.Intent.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return remediation.IntentRecord{}, err
	}
	return value, nil
}

func (s *Store) createApproval(ctx context.Context, value remediation.Approval) error {
	if !value.Valid() {
		return common.NewError(common.InvalidArgument, "approval is invalid", false)
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO approvals (id, tenant_id, org_id, task_id, intent_id, intent_digest, status, requested_at, expires_at, decided_at, decided_by, decision_reason, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.OrgID, value.TaskID, value.IntentID, value.IntentDigest, value.Status, storageTimestamp(value.RequestedAt), storageTimestamp(value.ExpiresAt), nullableTimestamp(value.DecidedAt), nullableString(value.DecidedBy), nullableString(value.DecisionReason), value.Version)
	return mapError(err)
}

func (s *Store) getApprovalByTask(ctx context.Context, tenantID, taskID string) (remediation.Approval, error) {
	if tenantID == "" || taskID == "" {
		return remediation.Approval{}, common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	row := s.executor().QueryRowContext(ctx, `SELECT id, tenant_id, org_id, task_id, intent_id, intent_digest, status, requested_at, expires_at, decided_at, decided_by, decision_reason, version FROM approvals WHERE tenant_id = ? AND task_id = ?`, tenantID, taskID)
	return scanApproval(row)
}

func (s *Store) updateApproval(ctx context.Context, value remediation.Approval, expectedVersion int64) error {
	if !value.Valid() || expectedVersion < 1 || value.Version != expectedVersion+1 || value.Status == remediation.ApprovalPending {
		return common.NewError(common.InvalidArgument, "approval update is invalid", false)
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE approvals SET status = ?, decided_at = ?, decided_by = ?, decision_reason = ?, version = ? WHERE tenant_id = ? AND id = ? AND version = ?`, value.Status, nullableTimestamp(value.DecidedAt), nullableString(value.DecidedBy), nullableString(value.DecisionReason), value.Version, value.TenantID, value.ID, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "approval")
}

func (s *Store) createRemediationExecution(ctx context.Context, value remediation.Execution) error {
	if !value.Valid() || value.State != remediation.ExecutionStarted {
		return common.NewError(common.InvalidArgument, "remediation execution is invalid", false)
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO remediation_executions (operation_id, tenant_id, org_id, task_id, approval_id, intent_digest, instance_epoch, expected_version, state, before_version, after_version, error_code, started_at, completed_at, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.OperationID, value.TenantID, value.OrgID, value.TaskID, value.ApprovalID, value.IntentDigest, value.InstanceEpoch, value.ExpectedVersion, value.State, nullableInt64(value.BeforeVersion), nullableInt64(value.AfterVersion), nullableDomainErrorCode(value.ErrorCode), storageTimestamp(value.StartedAt), nullableTimestamp(value.CompletedAt), value.Version)
	return mapError(err)
}

func (s *Store) getRemediationExecutionByTask(ctx context.Context, tenantID, taskID string) (remediation.Execution, error) {
	if tenantID == "" || taskID == "" {
		return remediation.Execution{}, common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	row := s.executor().QueryRowContext(ctx, `SELECT operation_id, tenant_id, org_id, task_id, approval_id, intent_digest, instance_epoch, expected_version, state, before_version, after_version, error_code, started_at, completed_at, version FROM remediation_executions WHERE tenant_id = ? AND task_id = ?`, tenantID, taskID)
	return scanRemediationExecution(row)
}

func (s *Store) getRemediationExecutionByOperation(ctx context.Context, tenantID, operationID string) (remediation.Execution, error) {
	if tenantID == "" || operationID == "" {
		return remediation.Execution{}, common.NewError(common.InvalidArgument, "tenant and operation are required", false)
	}
	row := s.executor().QueryRowContext(ctx, `SELECT operation_id, tenant_id, org_id, task_id, approval_id, intent_digest, instance_epoch, expected_version, state, before_version, after_version, error_code, started_at, completed_at, version FROM remediation_executions WHERE tenant_id = ? AND operation_id = ?`, tenantID, operationID)
	return scanRemediationExecution(row)
}

func (s *Store) updateRemediationExecution(ctx context.Context, value remediation.Execution, expectedVersion int64) error {
	if !value.Valid() || expectedVersion < 1 || value.Version != expectedVersion+1 || value.State == remediation.ExecutionStarted {
		return common.NewError(common.InvalidArgument, "remediation execution update is invalid", false)
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE remediation_executions SET state = ?, before_version = ?, after_version = ?, error_code = ?, completed_at = ?, version = ? WHERE tenant_id = ? AND operation_id = ? AND version = ?`, value.State, nullableInt64(value.BeforeVersion), nullableInt64(value.AfterVersion), nullableDomainErrorCode(value.ErrorCode), nullableTimestamp(value.CompletedAt), value.Version, value.TenantID, value.OperationID, expectedVersion)
	if err != nil {
		return mapError(err)
	}
	return requireUpdated(result, "remediation execution")
}

func (s *Store) createAuditRecord(ctx context.Context, value remediation.AuditRecord) error {
	if !value.Valid() {
		return common.NewError(common.InvalidArgument, "audit record is invalid", false)
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO audit_records (id, tenant_id, org_id, task_id, actor_id, action, outcome, summary, occurred_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.OrgID, value.TaskID, value.ActorID, value.Action, value.Outcome, value.Summary, storageTimestamp(value.OccurredAt))
	return mapError(err)
}

func (s *Store) listAuditRecordsByTask(ctx context.Context, tenantID, taskID string) ([]remediation.AuditRecord, error) {
	if tenantID == "" || taskID == "" {
		return nil, common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	if err := s.ensureTask(ctx, tenantID, taskID); err != nil {
		return nil, err
	}
	rows, err := s.executor().QueryContext(ctx, `SELECT id, tenant_id, org_id, task_id, actor_id, action, outcome, summary, occurred_at FROM audit_records WHERE tenant_id = ? AND task_id = ? ORDER BY occurred_at, id`, tenantID, taskID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	values := make([]remediation.AuditRecord, 0)
	for rows.Next() {
		var value remediation.AuditRecord
		var occurredAt string
		if err := rows.Scan(&value.ID, &value.TenantID, &value.OrgID, &value.TaskID, &value.ActorID, &value.Action, &value.Outcome, &value.Summary, &occurredAt); err != nil {
			return nil, mapError(err)
		}
		if value.OccurredAt, err = parseTimestamp(occurredAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return values, nil
}

func (s *Store) createToolCall(ctx context.Context, value task.ToolCallRecord) error {
	if err := validateToolCall(value); err != nil {
		return err
	}
	if err := s.ensureTask(ctx, value.TenantID, value.TaskID); err != nil {
		return err
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO tool_calls (id, tenant_id, task_id, source_call_id, tool_name, tool_version, status, input_summary_json, output_summary_json, error_code, error_message, started_at, completed_at, duration_ms, version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.TaskID, value.SourceCallID, value.ToolName, value.ToolVersion, value.Status, jsonString(value.InputSummary), nullableJSON(value.OutputSummary), nullableErrorCode(value.Error), nullableErrorMessage(value.Error), storageTimestamp(value.StartedAt), nullableTimestamp(value.CompletedAt), nullableInt64(value.DurationMS), value.Version)
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
	rows, err := s.executor().QueryContext(ctx, `SELECT id, tenant_id, task_id, source_call_id, tool_name, tool_version, status, input_summary_json, output_summary_json, error_code, error_message, started_at, completed_at, duration_ms, version FROM tool_calls WHERE tenant_id = ? AND task_id = ? ORDER BY started_at, id`, tenantID, taskID)
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
	var actualFrom, actualTo any
	if value.ActualSampleRange != nil {
		actualFrom, actualTo = storageTimestamp(value.ActualSampleRange.From), storageTimestamp(value.ActualSampleRange.To)
	}
	_, err = s.executor().ExecContext(ctx, `INSERT INTO chart_executions (id, tenant_id, chart_id, query_ref_id, status, series_count, duration_ms, sample_from, sample_to, actual_sample_from, actual_sample_to, series_json, warnings_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.TenantID, value.ChartID, value.QueryRefID, value.Status, len(value.Series), value.DurationMS, storageTimestamp(value.SampleRange.From), storageTimestamp(value.SampleRange.To), actualFrom, actualTo, string(seriesJSON), string(warningsJSON), storageTimestamp(value.CreatedAt))
	return mapError(err)
}

func (s *Store) listChartExecutionsByChart(ctx context.Context, tenantID, chartID string) ([]chart.Execution, error) {
	if tenantID == "" || chartID == "" {
		return nil, common.NewError(common.InvalidArgument, "tenant and chart are required", false)
	}
	if err := s.ensureChart(ctx, tenantID, chartID); err != nil {
		return nil, err
	}
	rows, err := s.executor().QueryContext(ctx, `SELECT id, tenant_id, chart_id, query_ref_id, status, duration_ms, sample_from, sample_to, actual_sample_from, actual_sample_to, series_json, warnings_json, created_at FROM chart_executions WHERE tenant_id = ? AND chart_id = ? ORDER BY created_at, id`, tenantID, chartID)
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
	return s.replayEventsTo(ctx, tenantID, taskID, afterSequence, int64(^uint64(0)>>1), limit)
}

func (s *Store) replayEventsTo(ctx context.Context, tenantID, taskID string, afterSequence, targetSequence int64, limit int) ([]task.TaskEvent, error) {
	if tenantID == "" || taskID == "" || afterSequence < 0 || targetSequence < afterSequence || limit < 1 {
		return nil, common.NewError(common.InvalidArgument, "event replay parameters are invalid", false)
	}
	if limit > maxReplayBatch {
		limit = maxReplayBatch
	}
	if err := s.ensureTask(ctx, tenantID, taskID); err != nil {
		return nil, err
	}
	rows, err := s.executor().QueryContext(ctx, `SELECT event_id, tenant_id, task_id, session_id, sequence, type, timestamp, payload_json FROM task_events WHERE tenant_id = ? AND task_id = ? AND sequence > ? AND sequence <= ? ORDER BY sequence LIMIT ?`, tenantID, taskID, afterSequence, targetSequence, limit)
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

func (s *Store) ensureIncidentTask(ctx context.Context, tenantID, taskID string) error {
	var found int
	err := s.executor().QueryRowContext(ctx, `SELECT 1 FROM tasks WHERE tenant_id = ? AND id = ? AND kind = 'incident_remediation'`, tenantID, taskID).Scan(&found)
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
	if err := scanner.Scan(&value.ID, &value.TenantID, &value.OrgID, &value.Kind, &value.Title, &value.Status, &value.CreatedBy, &createdAt, &updatedAt, &value.Version); err != nil {
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

func scanApproval(scanner interface{ Scan(...any) error }) (remediation.Approval, error) {
	var value remediation.Approval
	var requestedAt, expiresAt string
	var decidedAt, decidedBy, reason sql.NullString
	if err := scanner.Scan(&value.ID, &value.TenantID, &value.OrgID, &value.TaskID, &value.IntentID, &value.IntentDigest, &value.Status, &requestedAt, &expiresAt, &decidedAt, &decidedBy, &reason, &value.Version); err != nil {
		return remediation.Approval{}, mapError(err)
	}
	var err error
	if value.RequestedAt, err = parseTimestamp(requestedAt); err != nil {
		return remediation.Approval{}, err
	}
	if value.ExpiresAt, err = parseTimestamp(expiresAt); err != nil {
		return remediation.Approval{}, err
	}
	if decidedAt.Valid {
		parsed, err := parseTimestamp(decidedAt.String)
		if err != nil {
			return remediation.Approval{}, err
		}
		value.DecidedAt = &parsed
	}
	if decidedBy.Valid {
		value.DecidedBy = &decidedBy.String
	}
	if reason.Valid {
		value.DecisionReason = &reason.String
	}
	if !value.Valid() {
		return remediation.Approval{}, common.NewError(common.InternalError, "stored approval is invalid", false)
	}
	return value, nil
}

func scanRemediationExecution(scanner interface{ Scan(...any) error }) (remediation.Execution, error) {
	var value remediation.Execution
	var beforeVersion, afterVersion sql.NullInt64
	var errorCode, completedAt sql.NullString
	var startedAt string
	if err := scanner.Scan(&value.OperationID, &value.TenantID, &value.OrgID, &value.TaskID, &value.ApprovalID, &value.IntentDigest, &value.InstanceEpoch, &value.ExpectedVersion, &value.State, &beforeVersion, &afterVersion, &errorCode, &startedAt, &completedAt, &value.Version); err != nil {
		return remediation.Execution{}, mapError(err)
	}
	var err error
	if value.StartedAt, err = parseTimestamp(startedAt); err != nil {
		return remediation.Execution{}, err
	}
	if beforeVersion.Valid {
		value.BeforeVersion = &beforeVersion.Int64
	}
	if afterVersion.Valid {
		value.AfterVersion = &afterVersion.Int64
	}
	if errorCode.Valid {
		code := common.ErrorCode(errorCode.String)
		value.ErrorCode = &code
	}
	if completedAt.Valid {
		parsed, err := parseTimestamp(completedAt.String)
		if err != nil {
			return remediation.Execution{}, err
		}
		value.CompletedAt = &parsed
	}
	if !value.Valid() {
		return remediation.Execution{}, common.NewError(common.InternalError, "stored remediation execution is invalid", false)
	}
	return value, nil
}

func scanTask(scanner interface{ Scan(...any) error }) (task.AnalysisTask, error) {
	var value task.AnalysisTask
	var createdAt, updatedAt string
	var datasource, from, to, viewsJSON, incidentJSON sql.NullString
	var step, window sql.NullInt64
	var errorCode, errorMessage, startedAt, completedAt sql.NullString
	if err := scanner.Scan(&value.ID, &value.TenantID, &value.Kind, &value.SessionID, &value.Status, &value.InputMessageID, &datasource, &from, &to, &viewsJSON, &step, &window, &incidentJSON, &value.LatestSequence, &errorCode, &errorMessage, &createdAt, &startedAt, &completedAt, &updatedAt, &value.Version); err != nil {
		return task.AnalysisTask{}, mapError(err)
	}
	var err error
	if value.Kind == task.KindMetricAnalysis {
		if !datasource.Valid || !from.Valid || !to.Valid || !viewsJSON.Valid || !step.Valid || !window.Valid {
			return task.AnalysisTask{}, common.NewError(common.InternalError, "stored metric task plan is incomplete", false)
		}
		value.DatasourceUID = datasource.String
		value.QueryPlan.StepSeconds, value.QueryPlan.CPURateWindowSeconds = int(step.Int64), int(window.Int64)
		if err := json.Unmarshal([]byte(viewsJSON.String), &value.QueryPlan.Views); err != nil {
			return task.AnalysisTask{}, common.NewError(common.InternalError, "stored task views are invalid", false)
		}
		if value.TimeRange.From, err = parseTimestamp(from.String); err != nil {
			return task.AnalysisTask{}, err
		}
		if value.TimeRange.To, err = parseTimestamp(to.String); err != nil {
			return task.AnalysisTask{}, err
		}
	} else if value.Kind == task.KindIncidentRemediation {
		if !incidentJSON.Valid || datasource.Valid || from.Valid || to.Valid || viewsJSON.Valid || step.Valid || window.Valid {
			return task.AnalysisTask{}, common.NewError(common.InternalError, "stored incident task plan is invalid", false)
		}
		var plan task.IncidentPlan
		if err := json.Unmarshal([]byte(incidentJSON.String), &plan); err != nil {
			return task.AnalysisTask{}, common.NewError(common.InternalError, "stored incident task plan is invalid", false)
		}
		value.IncidentPlan = &plan
	} else {
		return task.AnalysisTask{}, common.NewError(common.InternalError, "stored task kind is invalid", false)
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
	if err := scanner.Scan(&value.ID, &value.TenantID, &value.TaskID, &value.SourceCallID, &value.ToolName, &value.ToolVersion, &value.Status, &inputJSON, &outputJSON, &errorCode, &errorMessage, &startedAt, &completedAt, &duration, &value.Version); err != nil {
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
	var actualFrom, actualTo sql.NullString
	if err := scanner.Scan(&value.ID, &value.TenantID, &value.ChartID, &value.QueryRefID, &value.Status, &value.DurationMS, &from, &to, &actualFrom, &actualTo, &seriesJSON, &warningsJSON, &createdAt); err != nil {
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
	if actualFrom.Valid != actualTo.Valid {
		return chart.Execution{}, common.NewError(common.InternalError, "chart execution actual sample range is incomplete", false)
	}
	if actualFrom.Valid {
		fromValue, fromErr := parseTimestamp(actualFrom.String)
		toValue, toErr := parseTimestamp(actualTo.String)
		if fromErr != nil || toErr != nil {
			return chart.Execution{}, common.NewError(common.InternalError, "chart execution actual sample range is invalid", false)
		}
		bounds, boundsErr := common.NewTimeBounds(fromValue, toValue)
		if boundsErr != nil {
			return chart.Execution{}, boundsErr
		}
		value.ActualSampleRange = &bounds
	}
	if value.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return chart.Execution{}, err
	}
	return value, nil
}

func validateTask(value task.AnalysisTask) error {
	if value.ID == "" || value.TenantID == "" || value.SessionID == "" || value.InputMessageID == "" || value.LatestSequence < 0 || value.Version < 1 || !isTaskStatus(value.Status) {
		return common.NewError(common.InvalidArgument, "task is invalid", false)
	}
	switch value.Kind {
	case task.KindMetricAnalysis:
		if value.DatasourceUID == "" || !value.TimeRange.From.Before(value.TimeRange.To) || !value.QueryPlan.Valid() || value.IncidentPlan != nil {
			return common.NewError(common.InvalidArgument, "metric task plan is invalid", false)
		}
	case task.KindIncidentRemediation:
		if value.IncidentPlan == nil || value.DatasourceUID != "" || !value.TimeRange.From.IsZero() || !value.TimeRange.To.IsZero() || value.QueryPlan.Valid() {
			return common.NewError(common.InvalidArgument, "incident task plan is invalid", false)
		}
		encoded, err := json.Marshal(value.IncidentPlan)
		if err != nil || !isJSONObject(encoded) {
			return common.NewError(common.InvalidArgument, "incident task plan is invalid", false)
		}
	default:
		return common.NewError(common.InvalidArgument, "task kind is invalid", false)
	}
	return nil
}

func taskPlanStorage(value task.AnalysisTask) (datasource, from, to, views, step, window, incident any, err error) {
	if value.Kind == task.KindMetricAnalysis {
		return value.DatasourceUID, storageTimestamp(value.TimeRange.From), storageTimestamp(value.TimeRange.To), stringSliceJSON(value.QueryPlan.Views), value.QueryPlan.StepSeconds, value.QueryPlan.CPURateWindowSeconds, nil, nil
	}
	encoded, marshalErr := json.Marshal(value.IncidentPlan)
	if marshalErr != nil {
		return nil, nil, nil, nil, nil, nil, nil, mapError(marshalErr)
	}
	return nil, nil, nil, nil, nil, nil, string(encoded), nil
}

func validatePageRequest(value repositories.PageRequest, maximum int) (int, error) {
	if value.Limit < 1 || value.Limit > maximum || (value.CreatedAt == nil && value.ID != "") || (value.CreatedAt != nil && value.ID == "") {
		return 0, common.NewError(common.InvalidArgument, "page request is invalid", false)
	}
	return value.Limit, nil
}

func validateTaskCheckpoint(value task.Checkpoint) error {
	if value.TaskID == "" || value.TenantID == "" || value.OpaqueValue == "" || len(value.OpaqueValue) > 16*1024 || value.UpdatedAt.IsZero() || value.Version < 1 || !isIncidentPhase(value.Phase) {
		return common.NewError(common.InvalidArgument, "task checkpoint is invalid", false)
	}
	return nil
}

func isIncidentPhase(phase task.IncidentPhase) bool {
	switch phase {
	case task.PhaseLoadAssets, task.PhaseObserve, task.PhaseNeedsAgent, task.PhasePrepare, task.PhaseNeedsApproval, task.PhaseExecute, task.PhaseVerifyRuntime, task.PhaseVerifyMetrics, task.PhaseVerifyBusiness, task.PhaseCompleted, task.PhaseNoAction:
		return true
	default:
		return false
	}
}

func pageMessages(items []session.Message, limit int) repositories.Page[session.Message] {
	page := repositories.Page[session.Message]{Items: items}
	if len(page.Items) <= limit {
		return page
	}
	page.Items = page.Items[:limit]
	last := page.Items[len(page.Items)-1]
	page.HasMore = true
	page.NextAfter = &repositories.PageCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	return page
}

func pageTasks(items []task.AnalysisTask, limit int) repositories.Page[task.AnalysisTask] {
	page := repositories.Page[task.AnalysisTask]{Items: items}
	if len(page.Items) <= limit {
		return page
	}
	page.Items = page.Items[:limit]
	last := page.Items[len(page.Items)-1]
	page.HasMore = true
	page.NextAfter = &repositories.PageCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	return page
}

func validateToolCall(value task.ToolCallRecord) error {
	if value.ID == "" || value.TenantID == "" || value.TaskID == "" || value.SourceCallID == "" || value.ToolName == "" || value.ToolVersion != "v1" || value.Version < 1 || !isToolCallStatus(value.Status) || !json.Valid(value.InputSummary) {
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
		if query.RefID == "" || query.Expression == "" || query.Legend == "" || query.DatasourceUID == "" || !query.TimeRange.From.Before(query.TimeRange.To) || !validQueryStep(query.StepSeconds) {
			return common.NewError(common.InvalidArgument, "chart query is invalid", false)
		}
	}
	return nil
}

func validateExecution(value chart.Execution) error {
	if value.ID == "" || value.TenantID == "" || value.ChartID == "" || value.QueryRefID == "" || value.DurationMS < 0 || !value.SampleRange.From.Before(value.SampleRange.To) || (value.Status != chart.ExecutionSuccess && value.Status != chart.ExecutionFailed) {
		return common.NewError(common.InvalidArgument, "chart execution is invalid", false)
	}
	if value.ActualSampleRange != nil && (value.ActualSampleRange.From.IsZero() || value.ActualSampleRange.To.IsZero() || value.ActualSampleRange.From.After(value.ActualSampleRange.To)) {
		return common.NewError(common.InvalidArgument, "chart execution actual sample range is invalid", false)
	}
	return nil
}

func validQueryStep(value int) bool {
	switch value {
	case 5, 10, 15, 30, 60, 120, 300:
		return true
	default:
		return false
	}
}

func validateEventDraft(value task.EventDraft) error {
	if value.EventID == "" || value.TenantID == "" || value.TaskID == "" || value.SessionID == "" || value.Timestamp.IsZero() || !isEventType(value.Type) || !isJSONObject(value.Payload) {
		return common.NewError(common.InvalidArgument, "task event is invalid", false)
	}
	return nil
}

func isTaskStatus(status task.Status) bool {
	return status == task.StatusCreated || status == task.StatusPlanning || status == task.StatusRunningTools || status == task.StatusWaitingApproval || status == task.StatusExecuting || status == task.StatusReconciling || status == task.StatusValidating || status == task.StatusCompleted || status == task.StatusFailed || status == task.StatusCancelled
}

func isToolCallStatus(status task.ToolCallStatus) bool {
	return status == task.ToolCallStarted || status == task.ToolCallCompleted || status == task.ToolCallFailed
}

func isEventType(eventType task.EventType) bool {
	switch eventType {
	case task.EventTaskCreated, task.EventTaskStatusChanged, task.EventAssistantMessageStarted, task.EventAssistantMessageDelta, task.EventAssistantMessageDone, task.EventToolStarted, task.EventToolCompleted, task.EventToolFailed, task.EventMetricCandidatesCreated, task.EventChartCreated, task.EventChartExecutionDone, task.EventTaskCompleted, task.EventTaskFailed, task.EventAlertReceived, task.EventPlaybookResolved, task.EventAssetsPinned, task.EventDiagnosisCompleted, task.EventIntentPrepared, task.EventApprovalRequested, task.EventApprovalDecided, task.EventRemediationStarted, task.EventRemediationReconciled, task.EventVerificationRuntime, task.EventVerificationMetrics, task.EventVerificationBusiness, task.EventAuditRecorded:
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

func nullableDomainErrorCode(value *common.ErrorCode) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func jsonString(value json.RawMessage) string { return string(value) }

func stringSliceJSON(value []string) string {
	if value == nil {
		return "[]"
	}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

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
