// Package workflows coordinates durable analysis state around the typed AgentRuntime.
package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
	"mini-torchbearing.local/services/ai-core/internal/ports/clocks"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
	"mini-torchbearing.local/services/ai-core/internal/ports/ids"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

type RunAnalysisWorkflow struct {
	Store    repositories.ApplicationStore
	Notifier events.Notifier
	Runtime  agent.Runtime
	IDs      ids.Generator
	Clock    clocks.Clock
}

func (w RunAnalysisWorkflow) Run(ctx context.Context, identity requestcontext.Context, taskID string) (err error) {
	if w.Store == nil || w.Runtime == nil || w.IDs == nil || w.Clock == nil {
		return common.NewError(common.InternalError, "analysis workflow is not configured", true)
	}
	if identity.TenantID == "" || taskID == "" {
		return common.NewError(common.InvalidArgument, "tenant and task are required", false)
	}
	item, err := w.Store.Tasks().Get(ctx, identity.TenantID, taskID)
	if err != nil {
		return err
	}
	if item.Status != task.StatusCreated {
		return common.NewError(common.InvalidStateTransition, "task has already been started", false)
	}
	defer func() {
		if err == nil {
			return
		}
		var domainErr *common.DomainError
		if !errors.As(err, &domainErr) {
			domainErr = common.NewError(common.InternalError, "analysis workflow failed", true)
		}
		_ = w.fail(ctx, &item, domainErr)
	}()

	if err = w.transition(ctx, &item, task.StatusPlanning); err != nil {
		return err
	}
	if err = w.transition(ctx, &item, task.StatusRunningTools); err != nil {
		return err
	}
	assistantMessageID := w.IDs.NewID("message")
	sink := &durableSink{workflow: w, identity: identity, task: &item, assistantMessageID: assistantMessageID, openCalls: make(map[string]task.ToolCallRecord)}
	result, err := w.Runtime.Run(ctx, identity, dto.AgentRunRequest{TaskID: item.ID, SessionID: item.SessionID, UserMessage: w.userMessage(ctx, item), DatasourceUID: item.DatasourceUID, TimeRange: item.TimeRange}, sink)
	if err != nil {
		return err
	}
	if err = w.transition(ctx, &item, task.StatusValidating); err != nil {
		return err
	}
	if err = w.persistResult(ctx, identity, item, assistantMessageID, result); err != nil {
		return err
	}
	if err = w.transition(ctx, &item, task.StatusCompleted); err != nil {
		return err
	}
	return w.emit(ctx, item, task.EventTaskCompleted, map[string]any{"task": taskSnapshot(item)})
}

func (w RunAnalysisWorkflow) userMessage(ctx context.Context, item task.AnalysisTask) string {
	messages, err := w.Store.Messages().ListBySession(ctx, item.TenantID, item.SessionID)
	if err != nil {
		return "analysis request"
	}
	for _, message := range messages {
		if message.ID == item.InputMessageID {
			return message.Content
		}
	}
	return "analysis request"
}

func (w RunAnalysisWorkflow) transition(ctx context.Context, item *task.AnalysisTask, next task.Status) error {
	latestSequence, err := w.Store.TaskEvents().LatestSequence(ctx, item.TenantID, item.ID)
	if err != nil {
		return err
	}
	item.LatestSequence = latestSequence
	previous := item.Status
	if err := item.Transition(next, w.Clock.Now()); err != nil {
		return err
	}
	if err := w.Store.Tasks().Update(ctx, *item, item.Version-1); err != nil {
		return err
	}
	return w.emit(ctx, *item, task.EventTaskStatusChanged, map[string]any{"previousStatus": previous, "status": next})
}

func (w RunAnalysisWorkflow) persistResult(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask, assistantMessageID string, result dto.AgentRunResult) error {
	message, err := session.NewMessage(assistantMessageID, item.TenantID, item.SessionID, session.RoleAssistant, result.AssistantText, w.Clock.Now())
	if err != nil {
		return err
	}
	if err := w.Store.Messages().Append(ctx, message); err != nil {
		return err
	}
	if err := w.emit(ctx, item, task.EventAssistantMessageDone, map[string]any{"message": map[string]any{"id": message.ID, "sessionId": message.SessionID, "role": message.Role, "content": message.Content, "createdAt": message.CreatedAt}}); err != nil {
		return err
	}
	for _, proposal := range result.Proposals {
		draft, err := chart.New(w.IDs.NewID("chart"), item.TenantID, item.SessionID, item.ID, proposal.Title, proposal.Unit, []chart.QuerySpec{proposal.Query}, w.Clock.Now())
		if err != nil {
			return err
		}
		if err := w.Store.Charts().Create(ctx, draft); err != nil {
			return err
		}
		if err := w.emit(ctx, item, task.EventChartCreated, map[string]any{"chart": map[string]any{"id": draft.ID, "taskId": draft.TaskID, "title": draft.Title, "status": draft.Status, "unit": draft.Unit, "visualization": draft.Visualization, "queries": querySpecsWire(draft.Queries)}}); err != nil {
			return err
		}
		execution := chart.Execution{ID: w.IDs.NewID("execution"), TenantID: item.TenantID, ChartID: draft.ID, QueryRefID: proposal.Query.RefID, Status: chart.ExecutionSuccess, Series: proposal.Execution.Series, DurationMS: proposal.Execution.DurationMS, SampleRange: item.TimeRange, Warnings: proposal.Execution.Warnings, CreatedAt: w.Clock.Now()}
		if err := w.Store.ChartExecutions().Create(ctx, execution); err != nil {
			return err
		}
		if err := draft.MarkReady(execution.ID, w.Clock.Now()); err != nil {
			return err
		}
		if err := w.Store.Charts().Update(ctx, draft, draft.Version-1); err != nil {
			return err
		}
		if err := w.emit(ctx, item, task.EventChartExecutionDone, map[string]any{"chartId": draft.ID, "chartStatus": draft.Status, "execution": executionWire(execution)}); err != nil {
			return err
		}
	}
	return nil
}

func (w RunAnalysisWorkflow) fail(ctx context.Context, item *task.AnalysisTask, domainErr *common.DomainError) error {
	if item.Status == task.StatusCompleted || item.Status == task.StatusFailed {
		return nil
	}
	latestSequence, err := w.Store.TaskEvents().LatestSequence(ctx, item.TenantID, item.ID)
	if err != nil {
		return err
	}
	item.LatestSequence = latestSequence
	if err := item.Fail(domainErr, w.Clock.Now()); err != nil {
		return err
	}
	if err := w.Store.Tasks().Update(ctx, *item, item.Version-1); err != nil {
		return err
	}
	return w.emit(ctx, *item, task.EventTaskFailed, map[string]any{"task": taskSnapshot(*item), "error": map[string]any{"code": domainErr.Code, "message": domainErr.Message, "retryable": domainErr.Retryable, "requestId": ""}})
}

func (w RunAnalysisWorkflow) emit(ctx context.Context, item task.AnalysisTask, kind task.EventType, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return common.NewError(common.InternalError, "cannot encode task event", false)
	}
	event, err := w.Store.TaskEvents().Append(ctx, task.EventDraft{EventID: w.IDs.NewID("event"), TenantID: item.TenantID, TaskID: item.ID, SessionID: item.SessionID, Type: kind, Timestamp: w.Clock.Now(), Payload: encoded})
	if err != nil {
		return err
	}
	if w.Notifier != nil {
		return w.Notifier.Notify(ctx, event)
	}
	return nil
}

type durableSink struct {
	workflow           RunAnalysisWorkflow
	identity           requestcontext.Context
	task               *task.AnalysisTask
	assistantMessageID string
	deltaOrdinal       int
	openCalls          map[string]task.ToolCallRecord
}

func (s *durableSink) Emit(ctx context.Context, source dto.AgentEvent) error {
	kind, ok := agentEventType(source.Type)
	if !ok {
		return common.NewError(common.InvalidArgument, "agent emitted an unsupported event", false)
	}
	payload := normalizePayload(source.Payload)
	if kind == task.EventAssistantMessageStarted {
		payload = map[string]any{"messageId": s.assistantMessageID, "role": "assistant"}
	}
	if kind == task.EventAssistantMessageDelta {
		s.deltaOrdinal++
		payload = map[string]any{"messageId": s.assistantMessageID, "delta": payload["content"], "ordinal": s.deltaOrdinal}
	}
	toolName, _ := payload["toolName"].(string)
	if kind == task.EventToolStarted && toolName != "" {
		encoded, _ := json.Marshal(payload)
		record := task.ToolCallRecord{ID: s.workflow.IDs.NewID("tool"), TenantID: s.task.TenantID, TaskID: s.task.ID, ToolName: toolName, ToolVersion: "v1", Status: task.ToolCallStarted, InputSummary: encoded, StartedAt: s.workflow.Clock.Now(), Version: 1}
		if err := s.workflow.Store.ToolCalls().Create(ctx, record); err != nil {
			return err
		}
		s.openCalls[toolName] = record
		payload = map[string]any{"toolCallId": record.ID, "toolName": toolName, "toolVersion": "v1", "inputSummary": payload}
	}
	if kind == task.EventToolCompleted && toolName != "" {
		if record, exists := s.openCalls[toolName]; exists {
			now := s.workflow.Clock.Now()
			duration := now.Sub(record.StartedAt).Milliseconds()
			if duration < 0 {
				duration = 0
			}
			record.Status, record.CompletedAt, record.DurationMS, record.Version = task.ToolCallCompleted, &now, &duration, record.Version+1
			encoded, _ := json.Marshal(payload)
			record.OutputSummary = encoded
			if err := s.workflow.Store.ToolCalls().Complete(ctx, record, record.Version-1); err != nil {
				return err
			}
			delete(s.openCalls, toolName)
			payload = map[string]any{"toolCallId": record.ID, "toolName": toolName, "durationMs": duration, "outputSummary": payload}
		}
	}
	return s.workflow.emit(ctx, *s.task, kind, payload)
}

func agentEventType(value string) (task.EventType, bool) {
	for _, kind := range []task.EventType{task.EventAssistantMessageStarted, task.EventAssistantMessageDelta, task.EventToolStarted, task.EventToolCompleted, task.EventMetricCandidatesCreated} {
		if string(kind) == value {
			return kind, true
		}
	}
	return "", false
}

func normalizePayload(value any) map[string]any {
	if text, ok := value.(string); ok {
		return map[string]any{"content": text}
	}
	if mapValue, ok := value.(map[string]any); ok {
		return mapValue
	}
	return map[string]any{}
}

func taskSnapshot(value task.AnalysisTask) map[string]any {
	return map[string]any{"id": value.ID, "sessionId": value.SessionID, "status": value.Status}
}

func executionWire(value chart.Execution) map[string]any {
	return map[string]any{"id": value.ID, "queryRefId": value.QueryRefID, "status": value.Status, "seriesCount": len(value.Series), "durationMs": value.DurationMS, "sampleRange": map[string]any{"from": value.SampleRange.From, "to": value.SampleRange.To}, "series": seriesWire(value.Series), "warnings": value.Warnings, "createdAt": value.CreatedAt}
}

func seriesWire(values []chart.Series) []map[string]any {
	series := make([]map[string]any, 0, len(values))
	for _, value := range values {
		points := make([]map[string]any, 0, len(value.Points))
		for _, point := range value.Points {
			points = append(points, map[string]any{"timestamp": point.Timestamp, "value": point.Value})
		}
		series = append(series, map[string]any{"name": value.Name, "labels": value.Labels, "points": points})
	}
	return series
}

func querySpecsWire(values []chart.QuerySpec) []map[string]any {
	queries := make([]map[string]any, 0, len(values))
	for _, value := range values {
		queries = append(queries, map[string]any{"refId": value.RefID, "expression": value.Expression, "legend": value.Legend, "datasourceUid": value.DatasourceUID, "timeRange": map[string]any{"from": value.TimeRange.From, "to": value.TimeRange.To}})
	}
	return queries
}

// Keep time imported in this file's public contract so implementations can use
// fixed clocks without leaking time.Now into the workflow.
var _ = time.Second
