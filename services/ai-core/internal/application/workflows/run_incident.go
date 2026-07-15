package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/clocks"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
	"mini-torchbearing.local/services/ai-core/internal/ports/ids"
	incidentport "mini-torchbearing.local/services/ai-core/internal/ports/incident"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

type RunIncidentWorkflow struct {
	Store    repositories.ApplicationStore
	Notifier events.Notifier
	Toolset  incidentport.Toolset
	IDs      ids.Generator
	Clock    clocks.Clock
}

func (w RunIncidentWorkflow) Run(ctx context.Context, identity requestcontext.Context, taskID string) error {
	if w.Store == nil || w.Toolset == nil || w.IDs == nil || w.Clock == nil || identity.TenantID == "" || taskID == "" {
		return common.NewError(common.InternalError, "Incident workflow is not configured", true)
	}
	item, err := w.Store.Tasks().Get(ctx, identity.TenantID, taskID)
	if err != nil {
		return err
	}
	if item.Kind != task.KindIncidentRemediation || item.IncidentPlan == nil {
		return common.NewError(common.InvalidArgument, "Task is not an Incident", false)
	}
	if item.Status == task.StatusCreated {
		if err := w.transition(ctx, &item, task.StatusPlanning); err != nil {
			return err
		}
	}
	if item.Status == task.StatusPlanning {
		if err := w.transition(ctx, &item, task.StatusRunningTools); err != nil {
			return err
		}
	}
	if item.Status != task.StatusRunningTools || item.IncidentPlan.Diagnosis != nil {
		return nil
	}
	observation, err := w.Toolset.Observe(ctx, identity, item.IncidentPlan.ServiceRef)
	if err != nil {
		return w.fail(ctx, item, err)
	}
	if err := validateDiagnosticEvidence(observation.Evidence); err != nil {
		return w.fail(ctx, item, err)
	}
	for _, evidence := range observation.Evidence {
		if err := w.persistToolEvidence(ctx, identity.TenantID, taskID, evidence); err != nil {
			return w.fail(ctx, item, err)
		}
	}
	item, err = w.Store.Tasks().Get(ctx, identity.TenantID, taskID)
	if err != nil {
		return err
	}
	if err := item.RecordDiagnosis(observation.Diagnosis, w.Clock.Now()); err != nil {
		return w.fail(ctx, item, err)
	}
	checkpoint, err := w.Store.TaskCheckpoints().Get(ctx, identity.TenantID, taskID)
	if err != nil {
		return w.fail(ctx, item, err)
	}
	if err := checkpoint.Replace(task.PhasePrepare, checkpoint.OpaqueValue, w.Clock.Now()); err != nil {
		return w.fail(ctx, item, err)
	}
	var event task.TaskEvent
	err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Tasks().Update(ctx, item, item.Version-1); err != nil {
			return err
		}
		if err := tx.TaskCheckpoints().Update(ctx, checkpoint, checkpoint.Version-1); err != nil {
			return err
		}
		var appendErr error
		event, appendErr = w.appendIncidentEvent(ctx, tx, item, task.EventDiagnosisCompleted, map[string]any{
			"primaryHypothesis": observation.Diagnosis.PrimaryHypothesis, "evidenceRefs": observation.Diagnosis.EvidenceRefs,
			"alternativeHypotheses": observation.Diagnosis.AlternativeHypotheses, "confidence": observation.Diagnosis.Confidence,
			"candidateAction": observation.Diagnosis.CandidateAction,
		})
		return appendErr
	})
	if err != nil {
		return err
	}
	w.notifyIncident(ctx, event)
	return nil
}

func validateDiagnosticEvidence(evidence []incidentport.ToolEvidence) error {
	allowed := map[string]bool{
		"order_service.get_queue_snapshot":  true,
		"order_service.get_worker_state":    true,
		"order_service.get_worker_policy":   true,
		"order_service.get_recent_outcomes": true,
	}
	if len(evidence) != len(allowed) {
		return common.NewError(common.InvalidStateTransition, "Incident Agent must use the bounded four-call diagnostic profile", false)
	}
	seen := make(map[string]bool, len(evidence))
	for _, item := range evidence {
		if !allowed[item.Name] || seen[item.Name] || item.DurationMS < 0 || !json.Valid(item.InputSummary) || !json.Valid(item.OutputSummary) {
			return common.NewError(common.InvalidStateTransition, "Incident Agent returned unsafe diagnostic evidence", false)
		}
		seen[item.Name] = true
	}
	return nil
}

func (w RunIncidentWorkflow) Recover(ctx context.Context, identityFor func(task.AnalysisTask) requestcontext.Context) error {
	items, err := w.Store.Tasks().ListNonTerminal(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Kind != task.KindIncidentRemediation || item.IncidentPlan == nil || item.IncidentPlan.Diagnosis != nil {
			continue
		}
		if err := w.Run(ctx, identityFor(item), item.ID); err != nil {
			return err
		}
	}
	return nil
}

func (w RunIncidentWorkflow) transition(ctx context.Context, item *task.AnalysisTask, next task.Status) error {
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
		var appendErr error
		event, appendErr = w.appendIncidentEvent(ctx, tx, candidate, task.EventTaskStatusChanged, map[string]any{"previousStatus": previous, "status": next})
		return appendErr
	})
	if err == nil {
		candidate.LatestSequence = event.Sequence
		*item = candidate
		w.notifyIncident(ctx, event)
	}
	return err
}

func (w RunIncidentWorkflow) persistToolEvidence(ctx context.Context, tenantID, taskID string, evidence incidentport.ToolEvidence) error {
	item, err := w.Store.Tasks().Get(ctx, tenantID, taskID)
	if err != nil {
		return err
	}
	completedAt := w.Clock.Now()
	startedAt := completedAt.Add(-time.Duration(evidence.DurationMS) * time.Millisecond)
	record := task.ToolCallRecord{ID: w.IDs.NewID("tool"), TenantID: tenantID, TaskID: taskID, SourceCallID: w.IDs.NewID("source"), ToolName: evidence.Name, ToolVersion: "v1", Status: task.ToolCallStarted, InputSummary: append(json.RawMessage(nil), evidence.InputSummary...), StartedAt: startedAt, Version: 1}
	var persisted []task.TaskEvent
	err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.ToolCalls().Create(ctx, record); err != nil {
			return err
		}
		started, err := w.appendIncidentEvent(ctx, tx, item, task.EventToolStarted, map[string]any{"toolCallId": record.ID, "toolName": record.ToolName, "toolVersion": "v1", "inputSummary": json.RawMessage(evidence.InputSummary)})
		if err != nil {
			return err
		}
		duration := evidence.DurationMS
		record.Status, record.OutputSummary, record.CompletedAt, record.DurationMS, record.Version = task.ToolCallCompleted, append(json.RawMessage(nil), evidence.OutputSummary...), &completedAt, &duration, 2
		if err := tx.ToolCalls().Complete(ctx, record, 1); err != nil {
			return err
		}
		completed, err := w.appendIncidentEvent(ctx, tx, item, task.EventToolCompleted, map[string]any{"toolCallId": record.ID, "toolName": record.ToolName, "durationMs": duration, "outputSummary": json.RawMessage(evidence.OutputSummary)})
		if err != nil {
			return err
		}
		persisted = []task.TaskEvent{started, completed}
		return nil
	})
	if err != nil {
		return err
	}
	for _, event := range persisted {
		w.notifyIncident(ctx, event)
	}
	return nil
}

func (w RunIncidentWorkflow) fail(ctx context.Context, item task.AnalysisTask, cause error) error {
	var domainErr *common.DomainError
	if !errors.As(cause, &domainErr) {
		domainErr = common.NewError(common.InternalError, "Incident workflow failed", true)
	}
	current, getErr := w.Store.Tasks().Get(ctx, item.TenantID, item.ID)
	if getErr != nil {
		return cause
	}
	if transitionErr := current.Fail(domainErr, w.Clock.Now()); transitionErr != nil {
		return cause
	}
	var eventsToNotify []task.TaskEvent
	_ = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Tasks().Update(ctx, current, current.Version-1); err != nil {
			return err
		}
		changed, err := w.appendIncidentEvent(ctx, tx, current, task.EventTaskStatusChanged, map[string]any{"previousStatus": item.Status, "status": task.StatusFailed, "error": eventError(domainErr)})
		if err != nil {
			return err
		}
		failed, err := w.appendIncidentEvent(ctx, tx, current, task.EventTaskFailed, map[string]any{"task": incidentTaskSnapshot(current), "error": eventError(domainErr)})
		if err == nil {
			eventsToNotify = []task.TaskEvent{changed, failed}
		}
		return err
	})
	for _, event := range eventsToNotify {
		w.notifyIncident(ctx, event)
	}
	return cause
}

func (w RunIncidentWorkflow) appendIncidentEvent(ctx context.Context, store repositories.ApplicationStore, item task.AnalysisTask, kind task.EventType, payload any) (task.TaskEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return task.TaskEvent{}, common.NewError(common.InternalError, "cannot encode Incident TaskEvent", false)
	}
	return store.TaskEvents().Append(ctx, task.EventDraft{EventID: w.IDs.NewID("event"), TenantID: item.TenantID, TaskID: item.ID, SessionID: item.SessionID, Type: kind, Timestamp: w.Clock.Now(), Payload: encoded})
}

func (w RunIncidentWorkflow) notifyIncident(ctx context.Context, event task.TaskEvent) {
	if w.Notifier != nil {
		_ = w.Notifier.Notify(ctx, event)
	}
}

func incidentTaskSnapshot(item task.AnalysisTask) map[string]any {
	return map[string]any{"id": item.ID, "kind": item.Kind, "sessionId": item.SessionID, "status": item.Status}
}

func eventError(err *common.DomainError) map[string]any {
	return map[string]any{"code": err.Code, "message": err.Message, "retryable": err.Retryable, "requestId": ""}
}
