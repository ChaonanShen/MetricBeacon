// Package commands owns request idempotency and the short synchronous portion
// of Session/Task creation. Execution begins only after the transaction commits.
package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/application/workflows"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
	"mini-torchbearing.local/services/ai-core/internal/ports/clocks"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
	"mini-torchbearing.local/services/ai-core/internal/ports/ids"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

const idempotencyTTL = 24 * time.Hour

type Service struct {
	Store    repositories.ApplicationStore
	Notifier events.Notifier
	Workflow workflows.RunAnalysisWorkflow
	IDs      ids.Generator
	Clock    clocks.Clock
	Planner  agent.IntentPlanner
	workers  chan struct{}
}

func New(store repositories.ApplicationStore, notifier events.Notifier, workflow workflows.RunAnalysisWorkflow, generator ids.Generator, clock clocks.Clock, planner agent.IntentPlanner) *Service {
	return &Service{Store: store, Notifier: notifier, Workflow: workflow, IDs: generator, Clock: clock, Planner: planner, workers: make(chan struct{}, 4)}
}

type CreateSessionInput struct{ Title, IdempotencyKey string }
type CreateTaskInput struct {
	SessionID, Message, DatasourceUID, IdempotencyKey string
	TimeRange                                         RequestedTimeRange
	StepSeconds                                       *int
	// RequestHash identifies the canonical caller intent before relative time
	// is resolved. In-process callers may leave it empty and use the fallback.
	RequestHash string
}

func (s *Service) CreateSession(ctx context.Context, identity requestcontext.Context, input CreateSessionInput) (session.AnalysisSession, error) {
	if s == nil || s.Store == nil || s.IDs == nil || s.Clock == nil {
		return session.AnalysisSession{}, common.NewError(common.InternalError, "command service is not configured", true)
	}
	if identity.TenantID == "" || identity.UserID == "" || strings.TrimSpace(input.Title) == "" || input.IdempotencyKey == "" {
		return session.AnalysisSession{}, common.NewError(common.InvalidArgument, "session identity, title and idempotency key are required", false)
	}
	var result session.AnalysisSession
	err := s.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		key := repositories.IdempotencyKey{TenantID: identity.TenantID, Scope: "create_session", Key: input.IdempotencyKey}
		record, err := tx.Idempotency().Reserve(ctx, key, hash(input), s.Clock.Now().Add(idempotencyTTL))
		if err != nil {
			return err
		}
		if record.Status == "completed" {
			result, err = tx.Sessions().Get(ctx, identity.TenantID, record.ResourceID)
			return err
		}
		result, err = session.New(s.IDs.NewID("session"), identity.TenantID, input.Title, identity.UserID, s.Clock.Now())
		if err != nil {
			return err
		}
		if err = tx.Sessions().Create(ctx, result); err != nil {
			return err
		}
		response, _ := json.Marshal(map[string]string{"id": result.ID})
		return tx.Idempotency().Complete(ctx, key, result.ID, response)
	})
	return result, err
}

func (s *Service) CreateTask(ctx context.Context, identity requestcontext.Context, input CreateTaskInput) (task.AnalysisTask, error) {
	if s == nil || s.Store == nil || s.IDs == nil || s.Clock == nil || s.Planner == nil {
		return task.AnalysisTask{}, common.NewError(common.InternalError, "command service is not configured", true)
	}
	if identity.TenantID == "" || strings.TrimSpace(input.SessionID) == "" || strings.TrimSpace(input.Message) == "" || utf8.RuneCountInString(input.Message) > 4_000 || strings.TrimSpace(input.DatasourceUID) == "" || input.IdempotencyKey == "" {
		return task.AnalysisTask{}, common.NewError(common.InvalidArgument, "task fields and idempotency key are required", false)
	}
	var result task.AnalysisTask
	shouldRun := false
	requestHash := strings.TrimSpace(input.RequestHash)
	if requestHash == "" {
		requestHash = hash(struct {
			TenantID, SessionID, Message, DatasourceUID string
			TimeRange                                   RequestedTimeRange
			StepSeconds                                 *int
		}{identity.TenantID, input.SessionID, input.Message, input.DatasourceUID, input.TimeRange, input.StepSeconds})
	}
	key := repositories.IdempotencyKey{TenantID: identity.TenantID, Scope: "create_task", Key: input.IdempotencyKey}
	if existing, getErr := s.Store.Idempotency().GetResult(ctx, key); getErr == nil {
		if existing.RequestHash != requestHash {
			return task.AnalysisTask{}, common.NewError(common.IdempotencyConflict, "idempotency key was already used with a different request", false)
		}
		if existing.Status == "completed" {
			return s.Store.Tasks().Get(ctx, identity.TenantID, existing.ResourceID)
		}
	} else if !hasErrorCode(getErr, common.ResourceNotFound) {
		return task.AnalysisTask{}, getErr
	}
	if _, err := s.Store.Sessions().Get(ctx, identity.TenantID, input.SessionID); err != nil {
		return task.AnalysisTask{}, err
	}
	history := s.planningHistory(ctx, identity.TenantID, input.SessionID)
	intent, err := s.Planner.Plan(ctx, identity, agent.IntentPlanRequest{Message: input.Message, History: history})
	if err != nil {
		return task.AnalysisTask{}, err
	}
	if intent.Status != agent.IntentPlanned && intent.Status != agent.IntentUnsupported {
		return task.AnalysisTask{}, common.NewError(common.DependencyUnavailable, "query intent planner returned an invalid status", true)
	}
	resolvedRange, queryPlan, err := ResolvePlannedQueryPlan(intent.Views, intent.RangeDuration, intent.StepSeconds, input.TimeRange, input.StepSeconds, s.Clock.Now())
	if err != nil {
		return task.AnalysisTask{}, err
	}
	err = s.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		record, err := tx.Idempotency().Reserve(ctx, key, requestHash, s.Clock.Now().Add(idempotencyTTL))
		if err != nil {
			return err
		}
		if record.Status == "completed" {
			result, err = tx.Tasks().Get(ctx, identity.TenantID, record.ResourceID)
			return err
		}
		shouldRun = true
		if _, err = tx.Sessions().Get(ctx, identity.TenantID, input.SessionID); err != nil {
			return err
		}
		taskID := s.IDs.NewID("task")
		message, err := session.NewMessage(s.IDs.NewID("message"), identity.TenantID, input.SessionID, taskID, session.RoleUser, input.Message, s.Clock.Now())
		if err != nil {
			return err
		}
		if err = tx.Messages().Append(ctx, message); err != nil {
			return err
		}
		result, err = task.New(taskID, identity.TenantID, input.SessionID, message.ID, input.DatasourceUID, resolvedRange, queryPlan, s.Clock.Now())
		if err != nil {
			return err
		}
		if err = tx.Tasks().Create(ctx, result); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"task": taskSnapshot(result)})
		event, err := tx.TaskEvents().Append(ctx, task.EventDraft{EventID: s.IDs.NewID("event"), TenantID: result.TenantID, TaskID: result.ID, SessionID: result.SessionID, Type: task.EventTaskCreated, Timestamp: s.Clock.Now(), Payload: payload})
		if err != nil {
			return err
		}
		result.LatestSequence = event.Sequence
		response, _ := json.Marshal(map[string]string{"id": result.ID})
		return tx.Idempotency().Complete(ctx, key, result.ID, response)
	})
	if err != nil {
		return task.AnalysisTask{}, err
	}
	if s.Notifier != nil {
		// The task-created event is durable. A prompt notification only wakes
		// already-connected SSE readers; they still replay from SQLite.
		latest, replayErr := s.Store.TaskEvents().Replay(ctx, result.TenantID, result.ID, result.LatestSequence-1, 1)
		if replayErr == nil && len(latest) == 1 {
			_ = s.Notifier.Notify(ctx, latest[0])
		}
	}
	if shouldRun {
		go s.run(identity, result.ID)
	}
	return result, nil
}

func (s *Service) run(identity requestcontext.Context, taskID string) {
	s.workers <- struct{}{}
	defer func() { <-s.workers }()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = s.Workflow.Run(ctx, identity, taskID)
}

func hash(value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func taskSnapshot(value task.AnalysisTask) map[string]any {
	return map[string]any{"id": value.ID, "sessionId": value.SessionID, "status": value.Status, "timeRange": map[string]any{"from": value.TimeRange.From, "to": value.TimeRange.To}, "queryPlan": map[string]any{"views": value.QueryPlan.Views, "stepSeconds": value.QueryPlan.StepSeconds, "cpuRateWindowSeconds": value.QueryPlan.CPURateWindowSeconds}}
}

func (s *Service) planningHistory(ctx context.Context, tenantID, sessionID string) []dto.ConversationMessage {
	messages, err := s.Store.Messages().ListBySession(ctx, tenantID, sessionID)
	if err != nil {
		return nil
	}
	history := make([]dto.ConversationMessage, 0, 12)
	characters := 0
	for index := len(messages) - 1; index >= 0 && len(history) < 12; index-- {
		count := utf8.RuneCountInString(messages[index].Content)
		if characters+count > 12_000 {
			break
		}
		characters += count
		history = append(history, dto.ConversationMessage{Role: string(messages[index].Role), Content: messages[index].Content})
	}
	for left, right := 0, len(history)-1; left < right; left, right = left+1, right-1 {
		history[left], history[right] = history[right], history[left]
	}
	return history
}

func hasErrorCode(err error, code common.ErrorCode) bool {
	var domainErr *common.DomainError
	return errors.As(err, &domainErr) && domainErr.Code == code
}
