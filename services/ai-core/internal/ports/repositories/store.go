package repositories

import (
	"context"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
)

type SessionRepository interface {
	Create(context.Context, session.AnalysisSession) error
	Get(context.Context, string, string) (session.AnalysisSession, error)
	Update(context.Context, session.AnalysisSession, int64) error
}
type MessageRepository interface {
	Append(context.Context, session.Message) error
	ListBySession(context.Context, string, string) ([]session.Message, error)
}
type TaskRepository interface {
	Create(context.Context, task.AnalysisTask) error
	Get(context.Context, string, string) (task.AnalysisTask, error)
	ListNonTerminal(context.Context) ([]task.AnalysisTask, error)
	Update(context.Context, task.AnalysisTask, int64) error
}
type ToolCallRepository interface {
	Create(context.Context, task.ToolCallRecord) error
	Complete(context.Context, task.ToolCallRecord, int64) error
	ListByTask(context.Context, string, string) ([]task.ToolCallRecord, error)
}
type ChartRepository interface {
	Create(context.Context, chart.ChartDraft) error
	Get(context.Context, string, string) (chart.ChartDraft, error)
	Update(context.Context, chart.ChartDraft, int64) error
	ListByTask(context.Context, string, string) ([]chart.ChartDraft, error)
}
type ChartExecutionRepository interface {
	Create(context.Context, chart.Execution) error
	ListByChart(context.Context, string, string) ([]chart.Execution, error)
}
type IdempotencyKey struct{ TenantID, Scope, Key string }
type IdempotencyRecord struct {
	Key                             IdempotencyKey
	RequestHash, Status, ResourceID string
	ResponseJSON                    []byte
	CreatedAt, ExpiresAt            time.Time
}
type IdempotencyRepository interface {
	Reserve(context.Context, IdempotencyKey, string, time.Time) (IdempotencyRecord, error)
	GetResult(context.Context, IdempotencyKey) (IdempotencyRecord, error)
	Complete(context.Context, IdempotencyKey, string, []byte) error
}
type ApplicationStore interface {
	Sessions() SessionRepository
	Messages() MessageRepository
	Tasks() TaskRepository
	ToolCalls() ToolCallRepository
	Charts() ChartRepository
	ChartExecutions() ChartExecutionRepository
	Idempotency() IdempotencyRepository
	TaskEvents() events.Store
	WithinTransaction(context.Context, func(ApplicationStore) error) error
	Health(context.Context) error
	Close() error
}
