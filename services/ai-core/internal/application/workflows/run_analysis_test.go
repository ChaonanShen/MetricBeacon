package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/events/inmemory"
	storage "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/storage/sqlite"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
	portevents "mini-torchbearing.local/services/ai-core/internal/ports/events"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

func TestRunPersistsOrderedEventsToolCallsAndCharts(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "ai-core.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	sessionValue, _ := session.New("session_1", "org:1", "Overview", "user:1", now)
	if err := store.Sessions().Create(ctx, sessionValue); err != nil {
		t.Fatal(err)
	}
	message, _ := session.NewMessage("message_1", "org:1", "session_1", "task_1", session.RoleUser, "show node exporter", now)
	taskValue, _ := task.New("task_1", "org:1", "session_1", "message_1", "prometheus-main", timeRange, task.LegacyQueryPlan(), now)
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Messages().Append(ctx, message); err != nil {
			return err
		}
		return tx.Tasks().Create(ctx, taskValue)
	}); err != nil {
		t.Fatal(err)
	}
	notifier := inmemory.New()
	workflow := RunAnalysisWorkflow{Store: store, Notifier: notifier, Runtime: scriptedRuntime{}, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
	if err := workflow.Run(ctx, requestcontext.Context{TenantID: "org:1", UserID: "user:1"}, "task_1"); err != nil {
		t.Fatal(err)
	}
	completed, err := store.Tasks().Get(ctx, "org:1", "task_1")
	if err != nil || completed.Status != task.StatusCompleted {
		t.Fatalf("task: %#v, %v", completed, err)
	}
	events, err := store.TaskEvents().Replay(ctx, "org:1", "task_1", 0, 200)
	if err != nil || len(events) < 10 {
		t.Fatalf("events: %#v, %v", events, err)
	}
	for index, event := range events {
		if event.Sequence != int64(index+1) {
			t.Fatalf("sequence at %d: %#v", index, event)
		}
	}
	charts, err := store.Charts().ListByTask(ctx, "org:1", "task_1")
	if err != nil || len(charts) != 3 {
		t.Fatalf("charts: %#v, %v", charts, err)
	}
	for _, draft := range charts {
		if draft.Status != chart.StatusReady || draft.LatestExecutionID == nil {
			t.Fatalf("chart was not ready: %#v", draft)
		}
	}
	calls, err := store.ToolCalls().ListByTask(ctx, "org:1", "task_1")
	if err != nil || len(calls) != 1 || calls[0].Status != task.ToolCallCompleted {
		t.Fatalf("tool calls: %#v, %v", calls, err)
	}
}

func TestRunUsesLiveCleanupContextAndPersistsFailureOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "ai-core.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	sessionValue, _ := session.New("session_1", "org:1", "Overview", "user:1", now)
	if err := store.Sessions().Create(context.Background(), sessionValue); err != nil {
		t.Fatal(err)
	}
	message, _ := session.NewMessage("message_1", "org:1", "session_1", "task_1", session.RoleUser, "show node exporter", now)
	taskValue, _ := task.New("task_1", "org:1", "session_1", "message_1", "prometheus-main", timeRange, task.LegacyQueryPlan(), now)
	if err := store.WithinTransaction(context.Background(), func(tx repositories.ApplicationStore) error {
		if err := tx.Messages().Append(context.Background(), message); err != nil {
			return err
		}
		return tx.Tasks().Create(context.Background(), taskValue)
	}); err != nil {
		t.Fatal(err)
	}

	workflow := RunAnalysisWorkflow{Store: store, Runtime: cancellingToolRuntime{cancel: cancel}, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
	if err := workflow.Run(ctx, requestcontext.Context{TenantID: "org:1", UserID: "user:1"}, "task_1"); err == nil {
		t.Fatal("workflow unexpectedly succeeded")
	}
	failed, err := store.Tasks().Get(context.Background(), "org:1", "task_1")
	if err != nil || failed.Status != task.StatusFailed || failed.Error == nil {
		t.Fatalf("failed task: %#v, %v", failed, err)
	}
	events, err := store.TaskEvents().Replay(context.Background(), "org:1", "task_1", 0, 20)
	if err != nil || len(events) < 2 {
		t.Fatalf("events: %#v, %v", events, err)
	}
	last := events[len(events)-3:]
	if last[0].Type != task.EventToolFailed || last[1].Type != task.EventTaskStatusChanged || last[2].Type != task.EventTaskFailed {
		t.Fatalf("failure event order: %s, %s, %s", last[0].Type, last[1].Type, last[2].Type)
	}
	if failed.LatestSequence != last[2].Sequence {
		t.Fatalf("latest sequence = %d, want %d", failed.LatestSequence, last[2].Sequence)
	}
	calls, err := store.ToolCalls().ListByTask(context.Background(), "org:1", "task_1")
	if err != nil || len(calls) != 1 || calls[0].Status != task.ToolCallFailed || calls[0].Error == nil {
		t.Fatalf("failed tool calls: %#v, %v", calls, err)
	}
}

func TestRecoverInterruptedFailsPersistedWorkWithoutRerun(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "ai-core.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	sessionValue, _ := session.New("session_1", "org:1", "Overview", "user:1", now)
	if err := store.Sessions().Create(ctx, sessionValue); err != nil {
		t.Fatal(err)
	}
	message, _ := session.NewMessage("message_1", "org:1", "session_1", "task_1", session.RoleUser, "show node exporter", now)
	taskValue, _ := task.New("task_1", "org:1", "session_1", "message_1", "prometheus-main", timeRange, task.LegacyQueryPlan(), now)
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Messages().Append(ctx, message); err != nil {
			return err
		}
		return tx.Tasks().Create(ctx, taskValue)
	}); err != nil {
		t.Fatal(err)
	}
	call := task.ToolCallRecord{ID: "tool_1", TenantID: "org:1", TaskID: "task_1", SourceCallID: "source_1", ToolName: "grafana.query_prometheus", ToolVersion: "v1", Status: task.ToolCallStarted, InputSummary: json.RawMessage(`{}`), StartedAt: now, Version: 1}
	if err := store.ToolCalls().Create(ctx, call); err != nil {
		t.Fatal(err)
	}

	workflow := RunAnalysisWorkflow{Store: store, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
	if err := workflow.RecoverInterrupted(ctx); err != nil {
		t.Fatal(err)
	}
	failed, err := store.Tasks().Get(ctx, "org:1", "task_1")
	if err != nil || failed.Status != task.StatusFailed || failed.Error == nil || failed.Error.Code != common.ExecutionInterrupted {
		t.Fatalf("recovered task: %#v, %v", failed, err)
	}
	events, err := store.TaskEvents().Replay(ctx, "org:1", "task_1", 0, 20)
	if err != nil || len(events) != 3 {
		t.Fatalf("recovery events: %#v, %v", events, err)
	}
	if events[0].Type != task.EventToolFailed || events[1].Type != task.EventTaskStatusChanged || events[2].Type != task.EventTaskFailed {
		t.Fatalf("recovery event order: %s, %s, %s", events[0].Type, events[1].Type, events[2].Type)
	}
	if err := workflow.RecoverInterrupted(ctx); err != nil {
		t.Fatal(err)
	}
	replayed, err := store.TaskEvents().Replay(ctx, "org:1", "task_1", 0, 20)
	if err != nil || len(replayed) != len(events) {
		t.Fatalf("recovery was not idempotent: %#v, %v", replayed, err)
	}
}

func TestTransitionRollsBackWhenEventAppendFails(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "ai-core.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	sessionValue, _ := session.New("session_1", "org:1", "Overview", "user:1", now)
	if err := store.Sessions().Create(ctx, sessionValue); err != nil {
		t.Fatal(err)
	}
	message, _ := session.NewMessage("message_1", "org:1", "session_1", "task_1", session.RoleUser, "show node exporter", now)
	taskValue, _ := task.New("task_1", "org:1", "session_1", "message_1", "prometheus-main", timeRange, task.LegacyQueryPlan(), now)
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Messages().Append(ctx, message); err != nil {
			return err
		}
		return tx.Tasks().Create(ctx, taskValue)
	}); err != nil {
		t.Fatal(err)
	}

	failingStore := eventFailingStore{ApplicationStore: store, failType: task.EventTaskStatusChanged}
	workflow := RunAnalysisWorkflow{Store: failingStore, Runtime: scriptedRuntime{}, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
	if err := workflow.Run(ctx, requestcontext.Context{TenantID: "org:1", UserID: "user:1"}, "task_1"); err == nil {
		t.Fatal("workflow unexpectedly succeeded")
	}
	persisted, err := store.Tasks().Get(ctx, "org:1", "task_1")
	if err != nil || persisted.Status != task.StatusCreated || persisted.Version != taskValue.Version {
		t.Fatalf("task mutation was not rolled back: %#v, %v", persisted, err)
	}
	events, err := store.TaskEvents().Replay(ctx, "org:1", "task_1", 0, 20)
	if err != nil || len(events) != 0 {
		t.Fatalf("unexpected events after rollback: %#v, %v", events, err)
	}
}

func TestConversationContextUsesPreviousTwelveMessagesInChronologicalOrder(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "ai-core.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	analysisSession, _ := session.New("session_context", "org:1", "Overview", "user:1", now)
	if err := store.Sessions().Create(ctx, analysisSession); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 13; index++ {
		createdAt := now.Add(time.Duration(index) * time.Second)
		taskID := fmt.Sprintf("task_prior_%02d", index)
		message, _ := session.NewMessage(fmt.Sprintf("message_prior_%02d", index), "org:1", analysisSession.ID, taskID, session.RoleUser, fmt.Sprintf("old-%02d", index), createdAt)
		item, _ := task.New(taskID, "org:1", analysisSession.ID, message.ID, "prometheus-main", timeRange, task.LegacyQueryPlan(), createdAt)
		item.Status = task.StatusCompleted
		if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
			if err := tx.Messages().Append(ctx, message); err != nil {
				return err
			}
			return tx.Tasks().Create(ctx, item)
		}); err != nil {
			t.Fatal(err)
		}
	}
	currentMessage, _ := session.NewMessage("message_current", "org:1", analysisSession.ID, "task_current", session.RoleUser, "current", now.Add(14*time.Second))
	currentTask, _ := task.New("task_current", "org:1", analysisSession.ID, currentMessage.ID, "prometheus-main", timeRange, task.LegacyQueryPlan(), now.Add(14*time.Second))
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Messages().Append(ctx, currentMessage); err != nil {
			return err
		}
		return tx.Tasks().Create(ctx, currentTask)
	}); err != nil {
		t.Fatal(err)
	}

	workflow := RunAnalysisWorkflow{Store: store}
	userMessage, history := workflow.conversationContext(ctx, currentTask)
	if userMessage != "current" || len(history) != 12 || history[0].Content != "old-02" || history[len(history)-1].Content != "old-13" {
		t.Fatalf("user=%q history=%#v", userMessage, history)
	}
}

type scriptedRuntime struct{}

func (scriptedRuntime) Run(ctx context.Context, _ requestcontext.Context, request dto.AgentRunRequest, sink agent.EventSink) (dto.AgentRunResult, error) {
	if err := sink.Emit(ctx, dto.AgentEvent{Type: string(task.EventAssistantMessageStarted), Payload: map[string]any{}}); err != nil {
		return dto.AgentRunResult{}, err
	}
	if err := sink.Emit(ctx, dto.AgentEvent{Type: string(task.EventToolStarted), SourceCallID: "scripted-01", Payload: map[string]any{"toolName": "grafana.query_prometheus"}}); err != nil {
		return dto.AgentRunResult{}, err
	}
	if err := sink.Emit(ctx, dto.AgentEvent{Type: string(task.EventToolCompleted), SourceCallID: "scripted-01", Payload: map[string]any{"toolName": "grafana.query_prometheus"}}); err != nil {
		return dto.AgentRunResult{}, err
	}
	proposals := make([]dto.ChartProposal, 0, 3)
	for _, value := range []struct{ key, title, unit string }{{"cpu", "CPU 使用率", "percent"}, {"memory", "内存可用率", "percent"}, {"load", "系统负载", "short"}} {
		proposals = append(proposals, dto.ChartProposal{Key: value.key, Title: value.title, Unit: value.unit, Query: chart.QuerySpec{RefID: value.key, Expression: "node_" + value.key, Legend: "{{instance}}", DatasourceUID: request.DatasourceUID, TimeRange: request.TimeRange, StepSeconds: request.QueryPlan.StepSeconds}, Execution: dto.QueryExecutionResult{Status: "success", Series: []chart.Series{{Name: "node-a"}}}})
	}
	return dto.AgentRunResult{AssistantText: "fixed result", Proposals: proposals}, nil
}

type cancellingToolRuntime struct{ cancel context.CancelFunc }

func (r cancellingToolRuntime) Run(ctx context.Context, _ requestcontext.Context, _ dto.AgentRunRequest, sink agent.EventSink) (dto.AgentRunResult, error) {
	if err := sink.Emit(ctx, dto.AgentEvent{Type: string(task.EventToolStarted), SourceCallID: "scripted-01", Payload: map[string]any{"toolName": "grafana.query_prometheus"}}); err != nil {
		return dto.AgentRunResult{}, err
	}
	r.cancel()
	return dto.AgentRunResult{}, ctx.Err()
}
func (cancellingToolRuntime) Resume(context.Context, requestcontext.Context, dto.AgentResumeRequest, agent.EventSink) (dto.AgentRunResult, error) {
	return dto.AgentRunResult{}, nil
}
func (scriptedRuntime) Resume(context.Context, requestcontext.Context, dto.AgentResumeRequest, agent.EventSink) (dto.AgentRunResult, error) {
	return dto.AgentRunResult{}, nil
}

type sequenceIDs struct{ value int }

func (s *sequenceIDs) NewID(kind string) string {
	s.value++
	return fmt.Sprintf("%s_%d", kind, s.value)
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type eventFailingStore struct {
	repositories.ApplicationStore
	failType task.EventType
}

func (s eventFailingStore) TaskEvents() portevents.Store {
	return eventFailingTaskEvents{Store: s.ApplicationStore.TaskEvents(), failType: s.failType}
}

func (s eventFailingStore) WithinTransaction(ctx context.Context, fn func(repositories.ApplicationStore) error) error {
	return s.ApplicationStore.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		return fn(eventFailingStore{ApplicationStore: tx, failType: s.failType})
	})
}

type eventFailingTaskEvents struct {
	portevents.Store
	failType task.EventType
}

func (s eventFailingTaskEvents) Append(ctx context.Context, draft task.EventDraft) (task.TaskEvent, error) {
	if draft.Type == s.failType {
		return task.TaskEvent{}, errors.New("injected task event failure")
	}
	return s.Store.Append(ctx, draft)
}
