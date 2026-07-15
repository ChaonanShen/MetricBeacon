package incidents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/workflows"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	domainincident "mini-torchbearing.local/services/ai-core/internal/domain/incident"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/clocks"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
	"mini-torchbearing.local/services/ai-core/internal/ports/ids"
	incidentport "mini-torchbearing.local/services/ai-core/internal/ports/incident"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

type Config struct {
	TenantID, OrgID, ActorID string
}

type Alert struct {
	SourceID, AlertName, Fingerprint, ServiceRef string
	Status                                       domainincident.AlertStatus
	Labels                                       map[string]string
	StartsAt                                     time.Time
	RequestID, TraceID                           string
}

type Result struct {
	TaskID    string
	Accepted  bool
	Duplicate bool
}

type Service struct {
	config   Config
	Store    repositories.ApplicationStore
	Notifier events.Notifier
	Toolset  incidentport.Toolset
	Workflow workflows.RunIncidentWorkflow
	IDs      ids.Generator
	Clock    clocks.Clock
	workers  chan struct{}
}

func New(config Config, store repositories.ApplicationStore, notifier events.Notifier, toolset incidentport.Toolset, workflow workflows.RunIncidentWorkflow, generator ids.Generator, clock clocks.Clock) *Service {
	return &Service{config: config, Store: store, Notifier: notifier, Toolset: toolset, Workflow: workflow, IDs: generator, Clock: clock, workers: make(chan struct{}, 4)}
}

func (s *Service) Ingest(ctx context.Context, input Alert) (Result, error) {
	if s == nil || s.Store == nil || s.Toolset == nil || s.IDs == nil || s.Clock == nil || s.config.TenantID == "" || s.config.OrgID == "" || s.config.ActorID == "" || input.SourceID == "" || input.AlertName == "" || input.Fingerprint == "" || input.ServiceRef == "" || input.StartsAt.IsZero() || len(input.Labels) == 0 {
		return Result{}, common.NewError(common.InvalidArgument, "alert ingestion request is invalid", false)
	}
	key := domainincident.AlertKey{TenantID: s.config.TenantID, OrgID: s.config.OrgID, SourceID: input.SourceID, Fingerprint: input.Fingerprint, StartsAt: input.StartsAt.UTC(), Status: input.Status}
	if existing, err := s.Store.AlertEvents().GetByKey(ctx, key); err == nil {
		return Result{TaskID: existing.TaskID, Duplicate: true}, nil
	} else if !hasCode(err, common.ResourceNotFound) {
		return Result{}, err
	}
	if input.Status == domainincident.AlertResolved {
		return s.recordResolved(ctx, input, key)
	}
	if input.Status != domainincident.AlertFiring {
		return Result{}, common.NewError(common.InvalidArgument, "alert status is invalid", false)
	}
	identity := s.identity(input.RequestID, input.TraceID)
	resolved, err := s.Toolset.ResolveAndStart(ctx, identity, input.SourceID, input.AlertName, cloneLabels(input.Labels))
	if err != nil {
		return Result{}, err
	}
	if resolved.ServiceRef != input.ServiceRef {
		return Result{}, common.NewError(common.SchemaValidationFailed, "alert mapping resolved a different service", false)
	}
	createdAt := s.Clock.Now()
	sessionID, taskID := s.IDs.NewID("session"), s.IDs.NewID("task")
	incidentSession, err := session.NewIncident(sessionID, s.config.TenantID, s.config.OrgID, fmt.Sprintf("%s · %s", input.AlertName, input.ServiceRef), s.config.ActorID, createdAt)
	if err != nil {
		return Result{}, err
	}
	trigger, err := session.NewMessage(s.IDs.NewID("message"), s.config.TenantID, sessionID, taskID, session.RoleTrigger, fmt.Sprintf("%s firing for %s", input.AlertName, input.ServiceRef), createdAt)
	if err != nil {
		return Result{}, err
	}
	plan := task.IncidentPlan{SourceID: input.SourceID, AlertName: input.AlertName, AlertFingerprint: input.Fingerprint, ServiceRef: resolved.ServiceRef, Labels: cloneLabels(input.Labels), Mapping: task.PinnedRef{ID: resolved.MappingID, Digest: resolved.MappingDigest}, Playbook: task.PinnedRef{ID: resolved.PlaybookID, Version: resolved.PlaybookVersion, Digest: resolved.PlaybookDigest}, AssetRefs: append([]task.AssetRef(nil), resolved.AssetRefs...), Phase: task.PhaseNeedsAgent}
	incidentTask, err := task.NewIncident(taskID, s.config.TenantID, sessionID, trigger.ID, plan, createdAt)
	if err != nil {
		return Result{}, err
	}
	checkpoint, err := task.NewCheckpoint(taskID, s.config.TenantID, task.PhaseNeedsAgent, resolved.Checkpoint, createdAt)
	if err != nil {
		return Result{}, err
	}
	alertEvent, err := domainincident.NewAlertEvent(s.IDs.NewID("alert"), key, input.ServiceRef, input.AlertName, input.Labels, taskID, createdAt)
	if err != nil {
		return Result{}, err
	}
	var persisted []task.TaskEvent
	err = s.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if existing, getErr := tx.AlertEvents().GetByKey(ctx, key); getErr == nil {
			incidentTask.ID = existing.TaskID
			return common.NewError(common.ResourceConflict, "alert already has an Incident Task", false)
		} else if !hasCode(getErr, common.ResourceNotFound) {
			return getErr
		}
		if err := tx.Sessions().Create(ctx, incidentSession); err != nil {
			return err
		}
		if err := tx.Messages().Append(ctx, trigger); err != nil {
			return err
		}
		if err := tx.Tasks().Create(ctx, incidentTask); err != nil {
			return err
		}
		if err := tx.TaskCheckpoints().Create(ctx, checkpoint); err != nil {
			return err
		}
		if err := tx.AlertEvents().Create(ctx, alertEvent); err != nil {
			return err
		}
		eventsToAppend := []struct {
			kind    task.EventType
			payload any
		}{
			{task.EventTaskCreated, map[string]any{"task": incidentSnapshot(incidentTask)}},
			{task.EventAlertReceived, map[string]any{"sourceId": input.SourceID, "alertName": input.AlertName, "fingerprint": input.Fingerprint, "serviceRef": input.ServiceRef, "status": input.Status, "startsAt": input.StartsAt.UTC()}},
			{task.EventPlaybookResolved, map[string]any{"mappingId": resolved.MappingID, "mappingDigest": resolved.MappingDigest, "playbookId": resolved.PlaybookID, "playbookVersion": resolved.PlaybookVersion, "playbookDigest": resolved.PlaybookDigest}},
			{task.EventAssetsPinned, map[string]any{"assets": resolved.AssetRefs}},
		}
		for _, draft := range eventsToAppend {
			encoded, _ := json.Marshal(draft.payload)
			event, err := tx.TaskEvents().Append(ctx, task.EventDraft{EventID: s.IDs.NewID("event"), TenantID: incidentTask.TenantID, TaskID: incidentTask.ID, SessionID: incidentTask.SessionID, Type: draft.kind, Timestamp: s.Clock.Now(), Payload: encoded})
			if err != nil {
				return err
			}
			persisted = append(persisted, event)
		}
		return nil
	})
	if err != nil {
		if hasCode(err, common.ResourceConflict) {
			if existing, getErr := s.Store.AlertEvents().GetByKey(ctx, key); getErr == nil {
				return Result{TaskID: existing.TaskID, Duplicate: true}, nil
			}
		}
		return Result{}, err
	}
	for _, event := range persisted {
		if s.Notifier != nil {
			_ = s.Notifier.Notify(ctx, event)
		}
	}
	s.schedule(identity, taskID)
	return Result{TaskID: taskID, Accepted: true}, nil
}

func (s *Service) recordResolved(ctx context.Context, input Alert, key domainincident.AlertKey) (Result, error) {
	firingKey := key
	firingKey.Status = domainincident.AlertFiring
	var taskID string
	if firing, err := s.Store.AlertEvents().GetByKey(ctx, firingKey); err == nil {
		taskID = firing.TaskID
	} else if !hasCode(err, common.ResourceNotFound) {
		return Result{}, err
	}
	event, err := domainincident.NewAlertEvent(s.IDs.NewID("alert"), key, input.ServiceRef, input.AlertName, input.Labels, taskID, s.Clock.Now())
	if err != nil {
		return Result{}, err
	}
	if err := s.Store.AlertEvents().Create(ctx, event); err != nil {
		if hasCode(err, common.ResourceConflict) {
			return Result{TaskID: taskID, Duplicate: true}, nil
		}
		return Result{}, err
	}
	return Result{TaskID: taskID, Accepted: true}, nil
}

func (s *Service) schedule(identity requestcontext.Context, taskID string) {
	go func() {
		s.workers <- struct{}{}
		defer func() { <-s.workers }()
		_ = s.Workflow.Run(context.Background(), identity, taskID)
	}()
}

func (s *Service) Recover(ctx context.Context) error {
	return s.Workflow.Recover(ctx, func(item task.AnalysisTask) requestcontext.Context {
		return s.identity("recovery:"+item.ID, "recovery:"+item.ID)
	})
}

func (s *Service) identity(requestID, traceID string) requestcontext.Context {
	return requestcontext.Context{TenantID: s.config.TenantID, OrgID: s.config.OrgID, UserID: s.config.ActorID, Roles: []string{"IncidentAgent"}, Permissions: []string{"incidents:diagnose"}, RequestID: requestID, TraceID: traceID}
}

func incidentSnapshot(item task.AnalysisTask) map[string]any {
	return map[string]any{"id": item.ID, "kind": item.Kind, "sessionId": item.SessionID, "status": item.Status}
}

func cloneLabels(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func hasCode(err error, code common.ErrorCode) bool {
	var value *common.DomainError
	return errors.As(err, &value) && value.Code == code
}
