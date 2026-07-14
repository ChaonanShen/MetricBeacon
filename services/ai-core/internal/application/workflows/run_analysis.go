// Package workflows coordinates durable analysis state around the typed AgentRuntime.
package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"
	"unicode/utf8"

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

// RecoverInterrupted fails persisted work that cannot safely be resumed after
// a process restart. It deliberately does not invoke the runtime.
func (w RunAnalysisWorkflow) RecoverInterrupted(ctx context.Context) error {
	if w.Store == nil || w.IDs == nil || w.Clock == nil {
		return common.NewError(common.InternalError, "analysis workflow is not configured", true)
	}
	items, err := w.Store.Tasks().ListNonTerminal(ctx)
	if err != nil {
		return err
	}
	interrupted := common.NewError(common.ExecutionInterrupted, "analysis execution was interrupted by process restart", true)
	for index := range items {
		item := &items[index]
		calls, err := w.Store.ToolCalls().ListByTask(ctx, item.TenantID, item.ID)
		if err != nil {
			return err
		}
		openCalls := make(map[string]task.ToolCallRecord)
		for _, call := range calls {
			if call.Status == task.ToolCallStarted {
				openCalls[call.ToolName+"#"+call.ID] = call
			}
		}
		if len(openCalls) > 0 {
			sink := durableSink{workflow: w, task: item, openCalls: openCalls}
			if err := sink.failOpenTools(ctx, interrupted); err != nil {
				return err
			}
		}
		if err := w.fail(ctx, item, interrupted); err != nil {
			return err
		}
	}
	return nil
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
	var sink *durableSink
	defer func() {
		if err == nil {
			return
		}
		var domainErr *common.DomainError
		if !errors.As(err, &domainErr) {
			domainErr = common.NewError(common.InternalError, "analysis workflow failed", true)
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if sink != nil {
			if toolErr := sink.failOpenTools(cleanupCtx, domainErr); toolErr != nil {
				err = errors.Join(err, toolErr)
			}
		}
		if failErr := w.fail(cleanupCtx, &item, domainErr); failErr != nil {
			err = errors.Join(err, failErr)
		}
	}()

	if err = w.transition(ctx, &item, task.StatusPlanning); err != nil {
		return err
	}
	if err = w.transition(ctx, &item, task.StatusRunningTools); err != nil {
		return err
	}
	assistantMessageID := w.IDs.NewID("message")
	sink = &durableSink{workflow: w, identity: identity, task: &item, assistantMessageID: assistantMessageID, openCalls: make(map[string]task.ToolCallRecord)}
	userMessage, history := w.conversationContext(ctx, item)
	result, err := w.Runtime.Run(ctx, identity, dto.AgentRunRequest{TaskID: item.ID, SessionID: item.SessionID, UserMessage: userMessage, DatasourceUID: item.DatasourceUID, TimeRange: item.TimeRange, History: history}, sink)
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

func (w RunAnalysisWorkflow) conversationContext(ctx context.Context, item task.AnalysisTask) (string, []dto.ConversationMessage) {
	messages, err := w.Store.Messages().ListBySession(ctx, item.TenantID, item.SessionID)
	if err != nil {
		return "analysis request", nil
	}
	current := -1
	for index, message := range messages {
		if message.ID == item.InputMessageID {
			current = index
			break
		}
	}
	if current < 0 {
		return "analysis request", nil
	}
	history := make([]dto.ConversationMessage, 0, 12)
	characters := 0
	for index := current - 1; index >= 0 && len(history) < 12; index-- {
		message := messages[index]
		if message.Role != session.RoleUser && message.Role != session.RoleAssistant {
			continue
		}
		count := utf8.RuneCountInString(message.Content)
		if characters+count > 12_000 {
			break
		}
		characters += count
		history = append(history, dto.ConversationMessage{Role: string(message.Role), Content: message.Content})
	}
	for left, right := 0, len(history)-1; left < right; left, right = left+1, right-1 {
		history[left], history[right] = history[right], history[left]
	}
	return messages[current].Content, history
}

func (w RunAnalysisWorkflow) transition(ctx context.Context, item *task.AnalysisTask, next task.Status) error {
	candidate := *item
	previous := candidate.Status
	var event task.TaskEvent
	err := w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		latestSequence, err := tx.TaskEvents().LatestSequence(ctx, candidate.TenantID, candidate.ID)
		if err != nil {
			return err
		}
		candidate.LatestSequence = latestSequence
		if err := candidate.Transition(next, w.Clock.Now()); err != nil {
			return err
		}
		if err := tx.Tasks().Update(ctx, candidate, candidate.Version-1); err != nil {
			return err
		}
		event, err = w.appendEvent(ctx, tx, candidate, task.EventTaskStatusChanged, map[string]any{"previousStatus": previous, "status": next})
		return err
	})
	if err != nil {
		return err
	}
	candidate.LatestSequence = event.Sequence
	*item = candidate
	w.notify(ctx, event)
	return nil
}

func (w RunAnalysisWorkflow) persistResult(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask, assistantMessageID string, result dto.AgentRunResult) error {
	message, err := session.NewMessage(assistantMessageID, item.TenantID, item.SessionID, item.ID, session.RoleAssistant, result.AssistantText, w.Clock.Now())
	if err != nil {
		return err
	}
	var messageEvent task.TaskEvent
	err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Messages().Append(ctx, message); err != nil {
			return err
		}
		var err error
		messageEvent, err = w.appendEvent(ctx, tx, item, task.EventAssistantMessageDone, map[string]any{"message": map[string]any{"id": message.ID, "sessionId": message.SessionID, "taskId": message.TaskID, "role": message.Role, "content": message.Content, "createdAt": message.CreatedAt}})
		return err
	})
	if err != nil {
		return err
	}
	w.notify(ctx, messageEvent)
	for _, proposal := range result.Proposals {
		draft, err := chart.New(w.IDs.NewID("chart"), item.TenantID, item.SessionID, item.ID, proposal.Title, proposal.Unit, []chart.QuerySpec{proposal.Query}, w.Clock.Now())
		if err != nil {
			return err
		}
		var chartEvent task.TaskEvent
		err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
			if err := tx.Charts().Create(ctx, draft); err != nil {
				return err
			}
			var err error
			chartEvent, err = w.appendEvent(ctx, tx, item, task.EventChartCreated, map[string]any{"chart": map[string]any{"id": draft.ID, "taskId": draft.TaskID, "title": draft.Title, "status": draft.Status, "unit": draft.Unit, "visualization": draft.Visualization, "queries": querySpecsWire(draft.Queries)}})
			return err
		})
		if err != nil {
			return err
		}
		w.notify(ctx, chartEvent)
		execution := chart.Execution{ID: w.IDs.NewID("execution"), TenantID: item.TenantID, ChartID: draft.ID, QueryRefID: proposal.Query.RefID, Status: chart.ExecutionSuccess, Series: proposal.Execution.Series, DurationMS: proposal.Execution.DurationMS, SampleRange: item.TimeRange, Warnings: proposal.Execution.Warnings, CreatedAt: w.Clock.Now()}
		if err := draft.MarkReady(execution.ID, w.Clock.Now()); err != nil {
			return err
		}
		var executionEvent task.TaskEvent
		err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
			if err := tx.ChartExecutions().Create(ctx, execution); err != nil {
				return err
			}
			if err := tx.Charts().Update(ctx, draft, draft.Version-1); err != nil {
				return err
			}
			var err error
			executionEvent, err = w.appendEvent(ctx, tx, item, task.EventChartExecutionDone, map[string]any{"chartId": draft.ID, "chartStatus": draft.Status, "execution": executionWire(execution)})
			return err
		})
		if err != nil {
			return err
		}
		w.notify(ctx, executionEvent)
	}
	return nil
}

func (w RunAnalysisWorkflow) fail(ctx context.Context, item *task.AnalysisTask, domainErr *common.DomainError) error {
	if item.Status == task.StatusCompleted || item.Status == task.StatusFailed {
		return nil
	}
	candidate := *item
	previous := candidate.Status
	eventsToNotify := make([]task.TaskEvent, 0, 2)
	err := w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		latestSequence, err := tx.TaskEvents().LatestSequence(ctx, candidate.TenantID, candidate.ID)
		if err != nil {
			return err
		}
		candidate.LatestSequence = latestSequence
		if err := candidate.Fail(domainErr, w.Clock.Now()); err != nil {
			return err
		}
		if err := tx.Tasks().Update(ctx, candidate, candidate.Version-1); err != nil {
			return err
		}
		statusEvent, err := w.appendEvent(ctx, tx, candidate, task.EventTaskStatusChanged, map[string]any{"previousStatus": previous, "status": task.StatusFailed})
		if err != nil {
			return err
		}
		failedEvent, err := w.appendEvent(ctx, tx, candidate, task.EventTaskFailed, map[string]any{"task": taskSnapshot(candidate), "error": map[string]any{"code": domainErr.Code, "message": domainErr.Message, "retryable": domainErr.Retryable, "requestId": ""}})
		if err != nil {
			return err
		}
		eventsToNotify = append(eventsToNotify, statusEvent, failedEvent)
		return nil
	})
	if err != nil {
		return err
	}
	candidate.LatestSequence = eventsToNotify[len(eventsToNotify)-1].Sequence
	*item = candidate
	for _, event := range eventsToNotify {
		w.notify(ctx, event)
	}
	return nil
}

func (w RunAnalysisWorkflow) emit(ctx context.Context, item task.AnalysisTask, kind task.EventType, payload any) error {
	event, err := w.appendEvent(ctx, w.Store, item, kind, payload)
	if err != nil {
		return err
	}
	w.notify(ctx, event)
	return nil
}

func (w RunAnalysisWorkflow) appendEvent(ctx context.Context, store repositories.ApplicationStore, item task.AnalysisTask, kind task.EventType, payload any) (task.TaskEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return task.TaskEvent{}, common.NewError(common.InternalError, "cannot encode task event", false)
	}
	return store.TaskEvents().Append(ctx, task.EventDraft{EventID: w.IDs.NewID("event"), TenantID: item.TenantID, TaskID: item.ID, SessionID: item.SessionID, Type: kind, Timestamp: w.Clock.Now(), Payload: encoded})
}

func (w RunAnalysisWorkflow) notify(ctx context.Context, event task.TaskEvent) {
	if w.Notifier != nil {
		_ = w.Notifier.Notify(ctx, event)
	}
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
	sourceCallID := source.SourceCallID
	if kind == task.EventToolStarted && toolName != "" {
		if sourceCallID == "" {
			return common.NewError(common.InvalidArgument, "agent tool start is missing source call id", false)
		}
		encoded, _ := json.Marshal(payload)
		record := task.ToolCallRecord{ID: s.workflow.IDs.NewID("tool"), TenantID: s.task.TenantID, TaskID: s.task.ID, SourceCallID: sourceCallID, ToolName: toolName, ToolVersion: "v1", Status: task.ToolCallStarted, InputSummary: encoded, StartedAt: s.workflow.Clock.Now(), Version: 1}
		payload = map[string]any{"toolCallId": record.ID, "toolName": toolName, "toolVersion": "v1", "inputSummary": payload}
		var event task.TaskEvent
		err := s.workflow.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
			if err := tx.ToolCalls().Create(ctx, record); err != nil {
				return err
			}
			var err error
			event, err = s.workflow.appendEvent(ctx, tx, *s.task, kind, payload)
			return err
		})
		if err != nil {
			return err
		}
		s.openCalls[sourceCallID] = record
		s.workflow.notify(ctx, event)
		return nil
	}
	if kind == task.EventToolCompleted && toolName != "" {
		if sourceCallID == "" {
			return common.NewError(common.InvalidArgument, "agent tool completion is missing source call id", false)
		}
		if record, exists := s.openCalls[sourceCallID]; exists {
			now := s.workflow.Clock.Now()
			duration := now.Sub(record.StartedAt).Milliseconds()
			if duration < 0 {
				duration = 0
			}
			record.Status, record.CompletedAt, record.DurationMS, record.Version = task.ToolCallCompleted, &now, &duration, record.Version+1
			encoded, _ := json.Marshal(payload)
			record.OutputSummary = encoded
			payload = map[string]any{"toolCallId": record.ID, "toolName": toolName, "durationMs": duration, "outputSummary": payload}
			var event task.TaskEvent
			err := s.workflow.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
				if err := tx.ToolCalls().Complete(ctx, record, record.Version-1); err != nil {
					return err
				}
				var err error
				event, err = s.workflow.appendEvent(ctx, tx, *s.task, kind, payload)
				return err
			})
			if err != nil {
				return err
			}
			delete(s.openCalls, sourceCallID)
			s.workflow.notify(ctx, event)
			return nil
		}
	}
	return s.workflow.emit(ctx, *s.task, kind, payload)
}

func (s *durableSink) failOpenTools(ctx context.Context, domainErr *common.DomainError) error {
	toolNames := make([]string, 0, len(s.openCalls))
	for toolName := range s.openCalls {
		toolNames = append(toolNames, toolName)
	}
	sort.Strings(toolNames)
	for _, openCallKey := range toolNames {
		record := s.openCalls[openCallKey]
		toolName := record.ToolName
		now := s.workflow.Clock.Now()
		duration := now.Sub(record.StartedAt).Milliseconds()
		if duration < 0 {
			duration = 0
		}
		record.Status, record.Error, record.CompletedAt, record.DurationMS, record.Version = task.ToolCallFailed, domainErr, &now, &duration, record.Version+1
		payload := map[string]any{"toolCallId": record.ID, "toolName": toolName, "durationMs": duration, "error": map[string]any{"code": domainErr.Code, "message": domainErr.Message, "retryable": domainErr.Retryable}}
		var event task.TaskEvent
		err := s.workflow.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
			if err := tx.ToolCalls().Complete(ctx, record, record.Version-1); err != nil {
				return err
			}
			var err error
			event, err = s.workflow.appendEvent(ctx, tx, *s.task, task.EventToolFailed, payload)
			return err
		})
		if err != nil {
			return err
		}
		delete(s.openCalls, openCallKey)
		s.workflow.notify(ctx, event)
	}
	return nil
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
