package workflows

import (
	"context"
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
	message, _ := session.NewMessage("message_1", "org:1", "session_1", session.RoleUser, "show node exporter", now)
	if err := store.Messages().Append(ctx, message); err != nil {
		t.Fatal(err)
	}
	taskValue, _ := task.New("task_1", "org:1", "session_1", "message_1", "mock-prometheus", timeRange, now)
	if err := store.Tasks().Create(ctx, taskValue); err != nil {
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

type scriptedRuntime struct{}

func (scriptedRuntime) Run(ctx context.Context, _ requestcontext.Context, request dto.AgentRunRequest, sink agent.EventSink) (dto.AgentRunResult, error) {
	if err := sink.Emit(ctx, dto.AgentEvent{Type: string(task.EventAssistantMessageStarted), Payload: map[string]any{}}); err != nil {
		return dto.AgentRunResult{}, err
	}
	if err := sink.Emit(ctx, dto.AgentEvent{Type: string(task.EventToolStarted), Payload: map[string]any{"toolName": "grafana.query_prometheus"}}); err != nil {
		return dto.AgentRunResult{}, err
	}
	if err := sink.Emit(ctx, dto.AgentEvent{Type: string(task.EventToolCompleted), Payload: map[string]any{"toolName": "grafana.query_prometheus"}}); err != nil {
		return dto.AgentRunResult{}, err
	}
	proposals := make([]dto.ChartProposal, 0, 3)
	for _, value := range []struct{ key, title, unit string }{{"cpu", "CPU 使用率", "percent"}, {"memory", "内存可用率", "percent"}, {"load", "系统负载", "short"}} {
		proposals = append(proposals, dto.ChartProposal{Key: value.key, Title: value.title, Unit: value.unit, Query: chart.QuerySpec{RefID: value.key, Expression: "node_" + value.key, Legend: "{{instance}}", DatasourceUID: request.DatasourceUID, TimeRange: request.TimeRange}, Execution: dto.QueryExecutionResult{Status: "success", Series: []chart.Series{{Name: "node-a"}}}})
	}
	return dto.AgentRunResult{AssistantText: "fixed result", Proposals: proposals}, nil
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
