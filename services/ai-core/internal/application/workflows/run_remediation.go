package workflows

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	approvalevidence "mini-torchbearing.local/packages/approval-evidence-go"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/remediation"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/clocks"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
	"mini-torchbearing.local/services/ai-core/internal/ports/ids"
	incidentport "mini-torchbearing.local/services/ai-core/internal/ports/incident"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

const (
	approvalEvidenceLifetime = 60 * time.Second
	defaultVerificationDelay = 5 * time.Second
	recoveryQueueLimit       = 10
	recoveryOldestAgeLimit   = 10
	probeDeadlineMS          = 5000
)

// RunRemediationWorkflow is the sole AI Core orchestration path allowed to
// turn an approved order Incident Intent into one typed write. Recovery never
// repeats that write: a durable operation ID is reconciled through read-only
// tools before verification resumes.
type RunRemediationWorkflow struct {
	Store                repositories.ApplicationStore
	Notifier             events.Notifier
	Toolset              incidentport.RemediationToolset
	Evidence             *approvalevidence.Codec
	IDs                  ids.Generator
	Clock                clocks.Clock
	VerificationInterval time.Duration
}

func (w RunRemediationWorkflow) RunApproved(ctx context.Context, identity requestcontext.Context, taskID string) error {
	if err := w.configured(identity, taskID); err != nil {
		return err
	}
	item, err := w.Store.Tasks().Get(ctx, identity.TenantID, taskID)
	if err != nil {
		return err
	}
	if item.Kind != task.KindIncidentRemediation || item.IncidentPlan == nil || item.IncidentPlan.Intent == nil {
		return common.NewError(common.InvalidArgument, "Task is not an approved Incident remediation", false)
	}
	switch item.Status {
	case task.StatusWaitingApproval:
		return w.execute(ctx, identity, item)
	case task.StatusExecuting:
		return w.recoverExecuting(ctx, identity, item)
	case task.StatusReconciling:
		execution, loadErr := w.Store.RemediationExecutions().GetByTask(ctx, item.TenantID, item.ID)
		if loadErr != nil {
			return loadErr
		}
		if execution.State == remediation.ExecutionStarted {
			if markErr := execution.MarkUnknown(w.Clock.Now()); markErr != nil {
				return markErr
			}
			if updateErr := w.Store.RemediationExecutions().Update(ctx, execution, execution.Version-1); updateErr != nil {
				return updateErr
			}
		}
		if execution.State == remediation.ExecutionUnknown {
			return w.reconcile(ctx, identity, item, execution)
		}
		return w.verifyRuntime(ctx, identity, item)
	case task.StatusValidating:
		checkpoint, loadErr := w.Store.TaskCheckpoints().Get(ctx, item.TenantID, item.ID)
		if loadErr != nil {
			return loadErr
		}
		if checkpoint.Phase == task.PhaseVerifyBusiness {
			return w.verifyBusiness(ctx, identity, item)
		}
		return w.verifyMetricsAndBusiness(ctx, identity, item)
	case task.StatusCompleted, task.StatusFailed, task.StatusCancelled:
		return nil
	default:
		return common.NewError(common.InvalidStateTransition, "Incident is not ready for remediation", false)
	}
}

func (w RunRemediationWorkflow) Recover(ctx context.Context, identityFor func(task.AnalysisTask) requestcontext.Context) error {
	if w.Store == nil || identityFor == nil {
		return common.NewError(common.InternalError, "remediation recovery is not configured", true)
	}
	items, err := w.Store.Tasks().ListNonTerminal(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Kind != task.KindIncidentRemediation || item.IncidentPlan == nil || item.IncidentPlan.Intent == nil {
			continue
		}
		if item.Status != task.StatusWaitingApproval && item.Status != task.StatusExecuting && item.Status != task.StatusReconciling && item.Status != task.StatusValidating {
			continue
		}
		approval, approvalErr := w.Store.Approvals().GetByTask(ctx, item.TenantID, item.ID)
		if approvalErr != nil {
			return approvalErr
		}
		if approval.Status != remediation.ApprovalApproved {
			continue
		}
		if err := w.RunApproved(ctx, identityFor(item), item.ID); err != nil {
			return err
		}
	}
	return nil
}

func (w RunRemediationWorkflow) execute(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask) error {
	intentRecord, approval, checkpoint, err := w.loadAuthority(ctx, identity, item)
	if err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationExecute, err)
	}
	now := w.Clock.Now().UTC()
	execution, err := remediation.NewExecution(w.IDs.NewID("operation"), item.TenantID, identity.OrgID, item.ID, approval.ID, intentRecord.Intent.Digest, intentRecord.Intent.InstanceEpoch, intentRecord.Intent.ExpectedVersion, now)
	if err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationExecute, err)
	}
	previous := item.Status
	if err := item.Transition(task.StatusExecuting, now); err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationExecute, err)
	}
	if err := checkpoint.Replace(task.PhaseExecute, checkpoint.OpaqueValue, now); err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationExecute, err)
	}
	audit, err := remediation.NewAuditRecord(w.IDs.NewID("audit"), item.TenantID, identity.OrgID, item.ID, identity.UserID, remediation.AuditRemediationExecute, remediation.AuditAccepted, "Approved typed worker concurrency remediation started", now)
	if err != nil {
		return err
	}
	var persisted []task.TaskEvent
	err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.RemediationExecutions().Create(ctx, execution); err != nil {
			return err
		}
		if err := tx.Tasks().Update(ctx, item, item.Version-1); err != nil {
			return err
		}
		if err := tx.TaskCheckpoints().Update(ctx, checkpoint, checkpoint.Version-1); err != nil {
			return err
		}
		if err := tx.AuditRecords().Create(ctx, audit); err != nil {
			return err
		}
		started, err := w.appendEvent(ctx, tx, item, task.EventRemediationStarted, map[string]any{"operationId": execution.OperationID, "approvalId": approval.ID, "intentDigest": intentRecord.Intent.Digest, "instanceEpoch": intentRecord.Intent.InstanceEpoch, "expectedVersion": intentRecord.Intent.ExpectedVersion})
		if err != nil {
			return err
		}
		audited, err := w.appendEvent(ctx, tx, item, task.EventAuditRecorded, map[string]any{"auditId": audit.ID, "action": audit.Action, "outcome": audit.Outcome})
		if err != nil {
			return err
		}
		changed, err := w.appendEvent(ctx, tx, item, task.EventTaskStatusChanged, map[string]any{"previousStatus": previous, "status": item.Status})
		if err != nil {
			return err
		}
		persisted = []task.TaskEvent{started, audited, changed}
		return nil
	})
	if err != nil {
		return err
	}
	w.notify(ctx, persisted...)

	token, err := w.Evidence.Sign(approvalevidence.Claims{Version: approvalevidence.Version, TenantID: item.TenantID, OrgID: identity.OrgID, TaskID: item.ID, ApprovalID: approval.ID, IntentDigest: intentRecord.Intent.Digest, CapabilityID: intentRecord.Intent.CapabilityID, ServiceRef: intentRecord.Intent.ServiceRef, InstanceEpoch: intentRecord.Intent.InstanceEpoch, ExpectedVersion: int(intentRecord.Intent.ExpectedVersion), OperationID: execution.OperationID, IssuedAt: now, ExpiresAt: now.Add(approvalEvidenceLifetime)})
	if err != nil {
		return w.makeUnknownAndReconcile(ctx, identity, item, execution, common.NewError(common.InternalError, "approval evidence could not be issued", false))
	}
	receipt, evidence, callErr := w.Toolset.RestoreWorkerConcurrency(ctx, identity, incidentport.RestoreRequest{OperationID: execution.OperationID, InstanceEpoch: execution.InstanceEpoch, ExpectedVersion: execution.ExpectedVersion, IntentDigest: execution.IntentDigest, ApprovalID: execution.ApprovalID, ApprovalEvidence: token})
	if evidence.Name != "" {
		if persistErr := w.persistToolEvidence(ctx, item, evidence, callErr); persistErr != nil {
			return w.makeUnknownAndReconcile(ctx, identity, item, execution, persistErr)
		}
	}
	if callErr != nil {
		return w.makeUnknownAndReconcile(ctx, identity, item, execution, callErr)
	}
	if err := validateReceipt(receipt, execution); err != nil {
		return w.makeUnknownAndReconcile(ctx, identity, item, execution, err)
	}
	return w.recordReceiptAndVerify(ctx, identity, item, execution, receipt, false)
}

func (w RunRemediationWorkflow) recoverExecuting(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask) error {
	execution, err := w.Store.RemediationExecutions().GetByTask(ctx, item.TenantID, item.ID)
	if err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationExecute, err)
	}
	if execution.State == remediation.ExecutionStarted {
		if err := execution.MarkUnknown(w.Clock.Now()); err != nil {
			return err
		}
		if err := w.Store.RemediationExecutions().Update(ctx, execution, execution.Version-1); err != nil {
			return err
		}
	}
	return w.reconcile(ctx, identity, item, execution)
}

func (w RunRemediationWorkflow) makeUnknownAndReconcile(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask, execution remediation.Execution, cause error) error {
	if execution.State == remediation.ExecutionStarted {
		if err := execution.MarkUnknown(w.Clock.Now()); err != nil {
			return err
		}
		if err := w.Store.RemediationExecutions().Update(ctx, execution, execution.Version-1); err != nil {
			return err
		}
	}
	return w.reconcile(ctx, identity, item, execution)
}

func (w RunRemediationWorkflow) reconcile(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask, execution remediation.Execution) error {
	if item.Status == task.StatusExecuting {
		if err := w.transitionWithCheckpoint(ctx, &item, task.StatusReconciling, task.PhaseVerifyRuntime); err != nil {
			return err
		}
	}
	receipt, evidence, err := w.Toolset.GetOperation(ctx, identity, execution.OperationID)
	if evidence.Name != "" {
		if persistErr := w.persistToolEvidence(ctx, item, evidence, err); persistErr != nil {
			return w.fail(ctx, identity, item, remediation.AuditRemediationExecute, persistErr)
		}
	}
	if err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationExecute, common.NewError(common.InvalidStateTransition, "remediation result is unknown and the durable operation receipt is unavailable", false))
	}
	if err := validateReceipt(receipt, execution); err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationExecute, err)
	}
	return w.recordReceiptAndVerify(ctx, identity, item, execution, receipt, true)
}

func (w RunRemediationWorkflow) recordReceiptAndVerify(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask, execution remediation.Execution, receipt incidentport.OperationReceipt, reconciled bool) error {
	if execution.State == remediation.ExecutionApplied || execution.State == remediation.ExecutionAlreadyApplied {
		return w.verifyRuntime(ctx, identity, item)
	}
	if err := execution.RecordReceipt(receipt.BeforeVersion, receipt.AfterVersion, reconciled, w.Clock.Now()); err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationExecute, err)
	}
	if err := w.syncLatestSequence(ctx, &item); err != nil {
		return err
	}
	taskChanged := false
	if item.Status == task.StatusExecuting {
		if err := item.Transition(task.StatusReconciling, w.Clock.Now()); err != nil {
			return err
		}
		taskChanged = true
	}
	checkpoint, err := w.Store.TaskCheckpoints().Get(ctx, item.TenantID, item.ID)
	if err != nil {
		return err
	}
	if err := checkpoint.Replace(task.PhaseVerifyRuntime, checkpoint.OpaqueValue, w.Clock.Now()); err != nil {
		return err
	}
	audit, err := remediation.NewAuditRecord(w.IDs.NewID("audit"), item.TenantID, identity.OrgID, item.ID, identity.UserID, remediation.AuditRemediationExecute, remediation.AuditSucceeded, fmt.Sprintf("Typed remediation receipt recorded: %s", execution.State), w.Clock.Now())
	if err != nil {
		return err
	}
	var persisted []task.TaskEvent
	err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.RemediationExecutions().Update(ctx, execution, execution.Version-1); err != nil {
			return err
		}
		if taskChanged {
			if err := tx.Tasks().Update(ctx, item, item.Version-1); err != nil {
				return err
			}
		}
		if err := tx.TaskCheckpoints().Update(ctx, checkpoint, checkpoint.Version-1); err != nil {
			return err
		}
		if err := tx.AuditRecords().Create(ctx, audit); err != nil {
			return err
		}
		reconciledEvent, err := w.appendEvent(ctx, tx, item, task.EventRemediationReconciled, map[string]any{"operationId": execution.OperationID, "state": execution.State, "beforeVersion": receipt.BeforeVersion, "afterVersion": receipt.AfterVersion, "beforeConcurrency": receipt.BeforeConcurrency, "afterConcurrency": receipt.AfterConcurrency})
		if err != nil {
			return err
		}
		audited, err := w.appendEvent(ctx, tx, item, task.EventAuditRecorded, map[string]any{"auditId": audit.ID, "action": audit.Action, "outcome": audit.Outcome})
		if err != nil {
			return err
		}
		persisted = []task.TaskEvent{reconciledEvent, audited}
		return nil
	})
	if err != nil {
		return err
	}
	w.notify(ctx, persisted...)
	return w.verifyRuntime(ctx, identity, item)
}

func (w RunRemediationWorkflow) verifyRuntime(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask) error {
	execution, err := w.Store.RemediationExecutions().GetByTask(ctx, item.TenantID, item.ID)
	if err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, err)
	}
	if execution.State != remediation.ExecutionApplied && execution.State != remediation.ExecutionAlreadyApplied {
		return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, common.NewError(common.InvalidStateTransition, "runtime verification requires an applied remediation", false))
	}
	runtimeState, runtimeEvidence, err := w.Toolset.GetRuntime(ctx, identity)
	if runtimeEvidence.Name != "" {
		if persistErr := w.persistToolEvidence(ctx, item, runtimeEvidence, err); persistErr != nil {
			return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, persistErr)
		}
	}
	if err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, err)
	}
	worker, workerEvidence, err := w.Toolset.GetWorker(ctx, identity)
	if workerEvidence.Name != "" {
		if persistErr := w.persistToolEvidence(ctx, item, workerEvidence, err); persistErr != nil {
			return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, persistErr)
		}
	}
	if err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, err)
	}
	if runtimeState.ServiceRef != item.IncidentPlan.ServiceRef || runtimeState.InstanceEpoch != execution.InstanceEpoch || runtimeState.SupervisorStatus != "running" || worker.ServiceRef != runtimeState.ServiceRef || worker.InstanceEpoch != execution.InstanceEpoch || worker.ConfiguredConcurrency != 2 || worker.EffectiveConcurrency != 2 || worker.ActiveWorkers < 1 || worker.Version != execution.ExpectedVersion+1 {
		return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, common.NewError(common.InvalidStateTransition, "runtime and worker state did not satisfy the approved Intent", false))
	}
	checkpoint, err := w.Store.TaskCheckpoints().Get(ctx, item.TenantID, item.ID)
	if err != nil {
		return err
	}
	if err := w.syncLatestSequence(ctx, &item); err != nil {
		return err
	}
	previous := item.Status
	if err := item.Transition(task.StatusValidating, w.Clock.Now()); err != nil {
		return err
	}
	if err := checkpoint.Replace(task.PhaseVerifyMetrics, checkpoint.OpaqueValue, w.Clock.Now()); err != nil {
		return err
	}
	var persisted []task.TaskEvent
	err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Tasks().Update(ctx, item, item.Version-1); err != nil {
			return err
		}
		if err := tx.TaskCheckpoints().Update(ctx, checkpoint, checkpoint.Version-1); err != nil {
			return err
		}
		runtimeEvent, err := w.appendEvent(ctx, tx, item, task.EventVerificationRuntime, map[string]any{"serviceRef": runtimeState.ServiceRef, "instanceEpoch": runtimeState.InstanceEpoch, "supervisorStatus": runtimeState.SupervisorStatus, "configuredConcurrency": worker.ConfiguredConcurrency, "effectiveConcurrency": worker.EffectiveConcurrency, "activeWorkers": worker.ActiveWorkers, "version": worker.Version, "observedAt": worker.ObservedAt})
		if err != nil {
			return err
		}
		changed, err := w.appendEvent(ctx, tx, item, task.EventTaskStatusChanged, map[string]any{"previousStatus": previous, "status": item.Status})
		if err != nil {
			return err
		}
		persisted = []task.TaskEvent{runtimeEvent, changed}
		return nil
	})
	if err != nil {
		return err
	}
	w.notify(ctx, persisted...)
	return w.verifyMetricsAndBusiness(ctx, identity, item)
}

func (w RunRemediationWorkflow) verifyMetricsAndBusiness(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask) error {
	first, evidence, err := w.Toolset.GetRecoveryMetrics(ctx, identity)
	if evidence.Name != "" {
		if persistErr := w.persistToolEvidence(ctx, item, evidence, err); persistErr != nil {
			return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, persistErr)
		}
	}
	if err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, err)
	}
	if err := w.wait(ctx); err != nil {
		return err
	}
	second, evidence, err := w.Toolset.GetRecoveryMetrics(ctx, identity)
	if evidence.Name != "" {
		if persistErr := w.persistToolEvidence(ctx, item, evidence, err); persistErr != nil {
			return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, persistErr)
		}
	}
	if err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, err)
	}
	if !validRecovery(first) || !validRecovery(second) || !second.ObservedAt.After(first.ObservedAt) {
		return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, common.NewError(common.InvalidStateTransition, "two consecutive recovery metric windows did not prove recovery", false))
	}
	checkpoint, err := w.Store.TaskCheckpoints().Get(ctx, item.TenantID, item.ID)
	if err != nil {
		return err
	}
	if err := checkpoint.Replace(task.PhaseVerifyBusiness, checkpoint.OpaqueValue, w.Clock.Now()); err != nil {
		return err
	}
	var metricsEvent task.TaskEvent
	err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.TaskCheckpoints().Update(ctx, checkpoint, checkpoint.Version-1); err != nil {
			return err
		}
		var appendErr error
		metricsEvent, appendErr = w.appendEvent(ctx, tx, item, task.EventVerificationMetrics, map[string]any{"windows": []incidentport.RecoveryMetrics{first, second}})
		return appendErr
	})
	if err != nil {
		return err
	}
	w.notify(ctx, metricsEvent)
	return w.verifyBusiness(ctx, identity, item)
}

func (w RunRemediationWorkflow) verifyBusiness(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask) error {
	probeID := stableProbeID(item.ID)
	probe, probeEvidence, err := w.Toolset.RunBusinessProbe(ctx, identity, probeID)
	if probeEvidence.Name != "" {
		if persistErr := w.persistToolEvidence(ctx, item, probeEvidence, err); persistErr != nil {
			return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, persistErr)
		}
	}
	if err != nil {
		return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, err)
	}
	if probe.ProbeID != probeID || probe.Result != "completed" || probe.DurationMS < 0 || probe.DurationMS > probeDeadlineMS || probe.CompletedAt.IsZero() {
		return w.fail(ctx, identity, item, remediation.AuditRemediationVerify, common.NewError(common.InvalidStateTransition, "business probe did not complete within the recovery bound", false))
	}
	return w.complete(ctx, identity, item, probe)
}

func validRecovery(value incidentport.RecoveryMetrics) bool {
	return value.WindowSeconds == 30 && value.AcceptedDelta >= 0 && value.CompletedDelta > 0 && value.CompletedDelta >= value.AcceptedDelta && value.QueueDepth >= 0 && value.QueueDepth <= recoveryQueueLimit && value.OldestAgeSeconds >= 0 && value.OldestAgeSeconds <= recoveryOldestAgeLimit && !value.ObservedAt.IsZero()
}

func validateReceipt(value incidentport.OperationReceipt, execution remediation.Execution) error {
	if value.OperationID != execution.OperationID || value.InstanceEpoch != execution.InstanceEpoch || value.IntentDigest != execution.IntentDigest || value.ApprovalID != execution.ApprovalID || value.BeforeVersion != execution.ExpectedVersion || value.AfterVersion != execution.ExpectedVersion+1 || value.BeforeConcurrency != 0 || value.AfterConcurrency != 2 || value.ExecutedAt.IsZero() || value.ExecutedAt.Before(execution.StartedAt) {
		return common.NewError(common.SchemaValidationFailed, "remediation receipt does not match the durable approved operation", false)
	}
	return nil
}

func (w RunRemediationWorkflow) complete(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask, probe incidentport.BusinessProbe) error {
	checkpoint, err := w.Store.TaskCheckpoints().Get(ctx, item.TenantID, item.ID)
	if err != nil {
		return err
	}
	previous := item.Status
	if err := w.syncLatestSequence(ctx, &item); err != nil {
		return err
	}
	if err := item.Transition(task.StatusCompleted, w.Clock.Now()); err != nil {
		return err
	}
	if err := checkpoint.Replace(task.PhaseCompleted, checkpoint.OpaqueValue, w.Clock.Now()); err != nil {
		return err
	}
	audit, err := remediation.NewAuditRecord(w.IDs.NewID("audit"), item.TenantID, identity.OrgID, item.ID, identity.UserID, remediation.AuditRemediationVerify, remediation.AuditSucceeded, "Runtime, Prometheus recovery windows, and business probe verified", w.Clock.Now())
	if err != nil {
		return err
	}
	var persisted []task.TaskEvent
	err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Tasks().Update(ctx, item, item.Version-1); err != nil {
			return err
		}
		if err := tx.TaskCheckpoints().Update(ctx, checkpoint, checkpoint.Version-1); err != nil {
			return err
		}
		if err := tx.AuditRecords().Create(ctx, audit); err != nil {
			return err
		}
		business, err := w.appendEvent(ctx, tx, item, task.EventVerificationBusiness, map[string]any{"probeId": probe.ProbeID, "result": probe.Result, "durationMs": probe.DurationMS, "completedAt": probe.CompletedAt})
		if err != nil {
			return err
		}
		audited, err := w.appendEvent(ctx, tx, item, task.EventAuditRecorded, map[string]any{"auditId": audit.ID, "action": audit.Action, "outcome": audit.Outcome})
		if err != nil {
			return err
		}
		changed, err := w.appendEvent(ctx, tx, item, task.EventTaskStatusChanged, map[string]any{"previousStatus": previous, "status": item.Status})
		if err != nil {
			return err
		}
		completed, err := w.appendEvent(ctx, tx, item, task.EventTaskCompleted, map[string]any{"task": incidentTaskSnapshot(item)})
		if err != nil {
			return err
		}
		persisted = []task.TaskEvent{business, audited, changed, completed}
		return nil
	})
	if err != nil {
		return err
	}
	w.notify(ctx, persisted...)
	return nil
}

func (w RunRemediationWorkflow) loadAuthority(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask) (remediation.IntentRecord, remediation.Approval, task.Checkpoint, error) {
	intent, err := w.Store.RemediationIntents().GetByTask(ctx, item.TenantID, item.ID)
	if err != nil {
		return remediation.IntentRecord{}, remediation.Approval{}, task.Checkpoint{}, err
	}
	approval, err := w.Store.Approvals().GetByTask(ctx, item.TenantID, item.ID)
	if err != nil {
		return remediation.IntentRecord{}, remediation.Approval{}, task.Checkpoint{}, err
	}
	checkpoint, err := w.Store.TaskCheckpoints().Get(ctx, item.TenantID, item.ID)
	if err != nil {
		return remediation.IntentRecord{}, remediation.Approval{}, task.Checkpoint{}, err
	}
	if approval.Status != remediation.ApprovalApproved || approval.OrgID != identity.OrgID || intent.OrgID != identity.OrgID || approval.IntentID != intent.Intent.ID || approval.IntentDigest != intent.Intent.Digest || item.IncidentPlan.Intent.Digest != intent.Intent.Digest || approval.DecidedAt == nil || approval.DecidedBy == nil || !approval.DecidedAt.Before(approval.ExpiresAt) {
		return remediation.IntentRecord{}, remediation.Approval{}, task.Checkpoint{}, common.NewError(common.ApprovalRequired, "a current exact-scope approval is required", false)
	}
	return intent, approval, checkpoint, nil
}

func (w RunRemediationWorkflow) transitionWithCheckpoint(ctx context.Context, item *task.AnalysisTask, next task.Status, phase task.IncidentPhase) error {
	checkpoint, err := w.Store.TaskCheckpoints().Get(ctx, item.TenantID, item.ID)
	if err != nil {
		return err
	}
	previous := item.Status
	if err := w.syncLatestSequence(ctx, item); err != nil {
		return err
	}
	if err := item.Transition(next, w.Clock.Now()); err != nil {
		return err
	}
	if err := checkpoint.Replace(phase, checkpoint.OpaqueValue, w.Clock.Now()); err != nil {
		return err
	}
	var event task.TaskEvent
	err = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Tasks().Update(ctx, *item, item.Version-1); err != nil {
			return err
		}
		if err := tx.TaskCheckpoints().Update(ctx, checkpoint, checkpoint.Version-1); err != nil {
			return err
		}
		var appendErr error
		event, appendErr = w.appendEvent(ctx, tx, *item, task.EventTaskStatusChanged, map[string]any{"previousStatus": previous, "status": next})
		return appendErr
	})
	if err == nil {
		w.notify(ctx, event)
	}
	return err
}

func (w RunRemediationWorkflow) syncLatestSequence(ctx context.Context, item *task.AnalysisTask) error {
	sequence, err := w.Store.TaskEvents().LatestSequence(ctx, item.TenantID, item.ID)
	if err != nil {
		return err
	}
	item.LatestSequence = sequence
	return nil
}

func (w RunRemediationWorkflow) persistToolEvidence(ctx context.Context, item task.AnalysisTask, evidence incidentport.ToolEvidence, callErr error) error {
	if evidence.Name == "" || evidence.DurationMS < 0 || !json.Valid(evidence.InputSummary) || (callErr == nil && !json.Valid(evidence.OutputSummary)) {
		return common.NewError(common.SchemaValidationFailed, "remediation tool evidence is invalid", false)
	}
	completedAt := w.Clock.Now()
	startedAt := completedAt.Add(-time.Duration(evidence.DurationMS) * time.Millisecond)
	record := task.ToolCallRecord{ID: w.IDs.NewID("tool"), TenantID: item.TenantID, TaskID: item.ID, SourceCallID: w.IDs.NewID("source"), ToolName: evidence.Name, ToolVersion: "v1", Status: task.ToolCallStarted, InputSummary: append(json.RawMessage(nil), evidence.InputSummary...), StartedAt: startedAt, Version: 1}
	var persisted []task.TaskEvent
	err := w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.ToolCalls().Create(ctx, record); err != nil {
			return err
		}
		started, err := w.appendEvent(ctx, tx, item, task.EventToolStarted, map[string]any{"toolCallId": record.ID, "toolName": record.ToolName, "toolVersion": "v1", "inputSummary": json.RawMessage(record.InputSummary)})
		if err != nil {
			return err
		}
		duration := evidence.DurationMS
		record.CompletedAt, record.DurationMS, record.Version = &completedAt, &duration, 2
		if callErr == nil {
			record.Status, record.OutputSummary = task.ToolCallCompleted, append(json.RawMessage(nil), evidence.OutputSummary...)
		} else {
			domainErr := asDomainError(callErr)
			record.Status, record.Error = task.ToolCallFailed, domainErr
		}
		if err := tx.ToolCalls().Complete(ctx, record, 1); err != nil {
			return err
		}
		kind := task.EventToolCompleted
		payload := map[string]any{"toolCallId": record.ID, "toolName": record.ToolName, "durationMs": duration, "outputSummary": json.RawMessage(record.OutputSummary)}
		if callErr != nil {
			kind = task.EventToolFailed
			payload = map[string]any{"toolCallId": record.ID, "toolName": record.ToolName, "durationMs": duration, "error": eventError(record.Error)}
		}
		finished, err := w.appendEvent(ctx, tx, item, kind, payload)
		if err != nil {
			return err
		}
		persisted = []task.TaskEvent{started, finished}
		return nil
	})
	if err == nil {
		w.notify(ctx, persisted...)
	}
	return err
}

func (w RunRemediationWorkflow) fail(ctx context.Context, identity requestcontext.Context, item task.AnalysisTask, action remediation.AuditAction, cause error) error {
	domainErr := asDomainError(cause)
	current, err := w.Store.Tasks().Get(ctx, item.TenantID, item.ID)
	if err != nil {
		return cause
	}
	if current.Status == task.StatusFailed || current.Status == task.StatusCancelled || current.Status == task.StatusCompleted {
		return cause
	}
	previous := current.Status
	if err := current.Fail(domainErr, w.Clock.Now()); err != nil {
		return cause
	}
	audit, err := remediation.NewAuditRecord(w.IDs.NewID("audit"), current.TenantID, identity.OrgID, current.ID, identity.UserID, action, remediation.AuditFailed, fmt.Sprintf("%s failed with %s", action, domainErr.Code), w.Clock.Now())
	if err != nil {
		return cause
	}
	var persisted []task.TaskEvent
	_ = w.Store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Tasks().Update(ctx, current, current.Version-1); err != nil {
			return err
		}
		if err := tx.AuditRecords().Create(ctx, audit); err != nil {
			return err
		}
		audited, err := w.appendEvent(ctx, tx, current, task.EventAuditRecorded, map[string]any{"auditId": audit.ID, "action": audit.Action, "outcome": audit.Outcome})
		if err != nil {
			return err
		}
		changed, err := w.appendEvent(ctx, tx, current, task.EventTaskStatusChanged, map[string]any{"previousStatus": previous, "status": task.StatusFailed, "error": eventError(domainErr)})
		if err != nil {
			return err
		}
		failed, err := w.appendEvent(ctx, tx, current, task.EventTaskFailed, map[string]any{"task": incidentTaskSnapshot(current), "error": eventError(domainErr)})
		if err != nil {
			return err
		}
		persisted = []task.TaskEvent{audited, changed, failed}
		return nil
	})
	w.notify(ctx, persisted...)
	return cause
}

func (w RunRemediationWorkflow) appendEvent(ctx context.Context, store repositories.ApplicationStore, item task.AnalysisTask, kind task.EventType, payload any) (task.TaskEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return task.TaskEvent{}, common.NewError(common.InternalError, "cannot encode remediation TaskEvent", false)
	}
	return store.TaskEvents().Append(ctx, task.EventDraft{EventID: w.IDs.NewID("event"), TenantID: item.TenantID, TaskID: item.ID, SessionID: item.SessionID, Type: kind, Timestamp: w.Clock.Now(), Payload: encoded})
}

func (w RunRemediationWorkflow) notify(ctx context.Context, values ...task.TaskEvent) {
	if w.Notifier == nil {
		return
	}
	for _, event := range values {
		_ = w.Notifier.Notify(ctx, event)
	}
}

func (w RunRemediationWorkflow) wait(ctx context.Context) error {
	delay := w.VerificationInterval
	if delay < 0 {
		return nil
	}
	if delay == 0 {
		delay = defaultVerificationDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func stableProbeID(taskID string) string {
	digest := sha256.Sum256([]byte(taskID))
	return fmt.Sprintf("probe-%x", digest[:])
}

func (w RunRemediationWorkflow) configured(identity requestcontext.Context, taskID string) error {
	if w.Store == nil || w.Toolset == nil || w.Evidence == nil || w.IDs == nil || w.Clock == nil {
		return common.NewError(common.InternalError, "remediation workflow is not configured", true)
	}
	if identity.TenantID == "" || identity.OrgID == "" || identity.UserID == "" || taskID == "" || !hasPermission(identity, "incidents:remediate") {
		return common.NewError(common.PermissionDenied, "Incident remediation permission is required", false)
	}
	return nil
}

func asDomainError(err error) *common.DomainError {
	var value *common.DomainError
	if errors.As(err, &value) {
		return value
	}
	return common.NewError(common.InternalError, "Incident remediation failed", true)
}

func hasPermission(identity requestcontext.Context, permission string) bool {
	for _, value := range identity.Permissions {
		if value == permission {
			return true
		}
	}
	return false
}
