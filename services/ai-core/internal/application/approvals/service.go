package approvals

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/remediation"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/clocks"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
	"mini-torchbearing.local/services/ai-core/internal/ports/ids"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

const idempotencyTTL = 24 * time.Hour

type ApprovedWorkflow interface {
	RunApproved(context.Context, requestcontext.Context, string) error
}

type Service struct {
	Store    repositories.ApplicationStore
	Notifier events.Notifier
	Workflow ApprovedWorkflow
	IDs      ids.Generator
	Clock    clocks.Clock
	workers  chan struct{}
}

type DecisionInput struct {
	TaskID, Decision, Reason, IntentDigest, IdempotencyKey string
	ExpectedTaskVersion, ExpectedApprovalVersion           int64
}

func New(store repositories.ApplicationStore, notifier events.Notifier, workflow ApprovedWorkflow, generator ids.Generator, clock clocks.Clock) *Service {
	return &Service{Store: store, Notifier: notifier, Workflow: workflow, IDs: generator, Clock: clock, workers: make(chan struct{}, 4)}
}

func (s *Service) Get(ctx context.Context, identity requestcontext.Context, taskID string) (remediation.Approval, error) {
	if err := s.configured(identity, taskID); err != nil {
		return remediation.Approval{}, err
	}
	if _, err := s.scopedTask(ctx, s.Store, identity, taskID); err != nil {
		return remediation.Approval{}, err
	}
	return s.Store.Approvals().GetByTask(ctx, identity.TenantID, taskID)
}

func (s *Service) Decide(ctx context.Context, identity requestcontext.Context, input DecisionInput) (remediation.Approval, error) {
	if err := s.configured(identity, input.TaskID); err != nil {
		return remediation.Approval{}, err
	}
	if !hasRole(identity, "Admin") {
		return remediation.Approval{}, common.NewError(common.PermissionDenied, "Admin role is required for Incident approval", false)
	}
	decision := remediation.Decision(input.Decision)
	if (decision != remediation.DecisionApprove && decision != remediation.DecisionReject) || len(input.Reason) > 500 || input.IdempotencyKey == "" || input.ExpectedTaskVersion < 1 || input.ExpectedApprovalVersion < 1 || !remediation.ValidDigest(input.IntentDigest) {
		return remediation.Approval{}, common.NewError(common.InvalidArgument, "approval decision is invalid", false)
	}
	requestHash := hashDecision(identity, input)
	key := repositories.IdempotencyKey{TenantID: identity.TenantID, Scope: "decide_approval:" + input.TaskID, Key: input.IdempotencyKey}
	var result remediation.Approval
	var persisted []task.TaskEvent
	approved := false
	err := s.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		record, err := tx.Idempotency().Reserve(ctx, key, requestHash, s.Clock.Now().Add(idempotencyTTL))
		if err != nil {
			return err
		}
		if record.Status == "completed" {
			result, err = tx.Approvals().GetByTask(ctx, identity.TenantID, input.TaskID)
			return err
		}
		item, err := s.scopedTask(ctx, tx, identity, input.TaskID)
		if err != nil {
			return err
		}
		if item.Status != task.StatusWaitingApproval || item.Version != input.ExpectedTaskVersion || item.IncidentPlan == nil || item.IncidentPlan.Intent == nil || item.IncidentPlan.Intent.Digest != input.IntentDigest {
			return common.NewError(common.ResourceConflict, "Task or immutable Intent changed before approval", false)
		}
		result, err = tx.Approvals().GetByTask(ctx, identity.TenantID, input.TaskID)
		if err != nil {
			return err
		}
		if result.Version != input.ExpectedApprovalVersion || result.IntentDigest != input.IntentDigest {
			return common.NewError(common.ResourceConflict, "Approval or immutable Intent changed before decision", false)
		}
		if err := result.Decide(decision, identity.UserID, input.Reason, s.Clock.Now()); err != nil {
			return err
		}
		if err := tx.Approvals().Update(ctx, result, input.ExpectedApprovalVersion); err != nil {
			return err
		}
		approved = result.Status == remediation.ApprovalApproved
		outcome := remediation.AuditAccepted
		if !approved {
			outcome = remediation.AuditRejected
			if err := item.Transition(task.StatusCancelled, s.Clock.Now()); err != nil {
				return err
			}
			if err := tx.Tasks().Update(ctx, item, input.ExpectedTaskVersion); err != nil {
				return err
			}
		}
		audit, err := remediation.NewAuditRecord(s.IDs.NewID("audit"), identity.TenantID, identity.OrgID, input.TaskID, identity.UserID, remediation.AuditApprovalDecision, outcome, fmt.Sprintf("Incident approval decision: %s", result.Status), s.Clock.Now())
		if err != nil {
			return err
		}
		if err := tx.AuditRecords().Create(ctx, audit); err != nil {
			return err
		}
		decided, err := appendEvent(ctx, tx, s.IDs, s.Clock, item, task.EventApprovalDecided, map[string]any{"approvalId": result.ID, "status": result.Status, "decidedBy": identity.UserID, "decidedAt": result.DecidedAt, "version": result.Version})
		if err != nil {
			return err
		}
		audited, err := appendEvent(ctx, tx, s.IDs, s.Clock, item, task.EventAuditRecorded, map[string]any{"auditId": audit.ID, "action": audit.Action, "outcome": audit.Outcome})
		if err != nil {
			return err
		}
		persisted = []task.TaskEvent{decided, audited}
		if !approved {
			changed, err := appendEvent(ctx, tx, s.IDs, s.Clock, item, task.EventTaskStatusChanged, map[string]any{"previousStatus": task.StatusWaitingApproval, "status": task.StatusCancelled})
			if err != nil {
				return err
			}
			persisted = append(persisted, changed)
		}
		response, _ := json.Marshal(map[string]any{"approvalId": result.ID, "status": result.Status, "version": result.Version})
		return tx.Idempotency().Complete(ctx, key, result.ID, response)
	})
	if err != nil {
		return remediation.Approval{}, err
	}
	for _, event := range persisted {
		if s.Notifier != nil {
			_ = s.Notifier.Notify(ctx, event)
		}
	}
	if approved && len(persisted) > 0 && s.Workflow != nil {
		s.schedule(identity, input.TaskID)
	}
	return result, nil
}

func (s *Service) configured(identity requestcontext.Context, taskID string) error {
	if s == nil || s.Store == nil || s.IDs == nil || s.Clock == nil {
		return common.NewError(common.InternalError, "approval service is not configured", true)
	}
	if identity.TenantID == "" || identity.OrgID == "" || identity.UserID == "" || taskID == "" {
		return common.NewError(common.InvalidArgument, "approval identity and Task are required", false)
	}
	return nil
}

func (s *Service) scopedTask(ctx context.Context, store repositories.ApplicationStore, identity requestcontext.Context, taskID string) (task.AnalysisTask, error) {
	item, err := store.Tasks().Get(ctx, identity.TenantID, taskID)
	if err != nil {
		return task.AnalysisTask{}, err
	}
	incidentSession, err := store.Sessions().Get(ctx, identity.TenantID, item.SessionID)
	if err != nil {
		return task.AnalysisTask{}, err
	}
	if item.Kind != task.KindIncidentRemediation || incidentSession.Kind != session.KindOrgIncident || incidentSession.OrgID != identity.OrgID {
		return task.AnalysisTask{}, common.NewError(common.ResourceNotFound, "Incident Task was not found", false)
	}
	return item, nil
}

func (s *Service) schedule(identity requestcontext.Context, taskID string) {
	go func() {
		s.workers <- struct{}{}
		defer func() { <-s.workers }()
		identity.Permissions = []string{"incidents:remediate"}
		_ = s.Workflow.RunApproved(context.Background(), identity, taskID)
	}()
}

func hasRole(identity requestcontext.Context, role string) bool {
	for _, candidate := range identity.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func hashDecision(identity requestcontext.Context, input DecisionInput) string {
	encoded, _ := json.Marshal(struct {
		TenantID, OrgID, TaskID, Decision, Reason, IntentDigest string
		ExpectedTaskVersion, ExpectedApprovalVersion            int64
	}{identity.TenantID, identity.OrgID, input.TaskID, input.Decision, input.Reason, input.IntentDigest, input.ExpectedTaskVersion, input.ExpectedApprovalVersion})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func appendEvent(ctx context.Context, store repositories.ApplicationStore, generator ids.Generator, clock clocks.Clock, item task.AnalysisTask, kind task.EventType, payload any) (task.TaskEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return task.TaskEvent{}, common.NewError(common.InternalError, "cannot encode approval TaskEvent", false)
	}
	return store.TaskEvents().Append(ctx, task.EventDraft{EventID: generator.NewID("event"), TenantID: item.TenantID, TaskID: item.ID, SessionID: item.SessionID, Type: kind, Timestamp: clock.Now(), Payload: encoded})
}
