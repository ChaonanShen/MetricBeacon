package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	storage "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/storage/sqlite"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/remediation"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	incidentport "mini-torchbearing.local/services/ai-core/internal/ports/incident"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

func TestRunIncidentPersistsBoundedEvidenceDiagnosisAndCheckpoint(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	seedIncidentTask(t, store, now, "task_1", task.StatusCreated)
	workflow := RunIncidentWorkflow{Store: store, Toolset: incidentWorkflowToolset{}, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}

	if err := workflow.Run(ctx, incidentIdentity(), "task_1"); err != nil {
		t.Fatal(err)
	}
	item, err := store.Tasks().Get(ctx, "org:1", "task_1")
	if err != nil || item.Status != task.StatusWaitingApproval || item.IncidentPlan == nil || item.IncidentPlan.Diagnosis == nil || item.IncidentPlan.Intent == nil {
		t.Fatalf("diagnosed task: %#v, %v", item, err)
	}
	if item.IncidentPlan.Phase != task.PhaseNeedsApproval || item.IncidentPlan.Diagnosis.PrimaryHypothesis != "worker_stopped" || item.IncidentPlan.Diagnosis.CandidateAction != "restore_worker_concurrency" || item.IncidentPlan.Intent.CapabilityID != "order_service.restore_worker_concurrency" {
		t.Fatalf("incident plan: %#v", item.IncidentPlan)
	}
	checkpoint, err := store.TaskCheckpoints().Get(ctx, "org:1", "task_1")
	if err != nil || checkpoint.Phase != task.PhaseNeedsApproval || checkpoint.OpaqueValue != "prepared-checkpoint" || checkpoint.Version != 3 {
		t.Fatalf("checkpoint: %#v, %v", checkpoint, err)
	}
	intentRecord, err := store.RemediationIntents().GetByTask(ctx, "org:1", "task_1")
	if err != nil || intentRecord.Intent.Digest != item.IncidentPlan.Intent.Digest || intentRecord.OrgID != "1" {
		t.Fatalf("intent record: %#v, %v", intentRecord, err)
	}
	approval, err := store.Approvals().GetByTask(ctx, "org:1", "task_1")
	if err != nil || approval.Status != remediation.ApprovalPending || approval.IntentDigest != item.IncidentPlan.Intent.Digest || !approval.ExpiresAt.Equal(now.Add(remediation.DefaultApprovalTTL)) {
		t.Fatalf("approval: %#v, %v", approval, err)
	}
	if _, err := store.RemediationExecutions().GetByTask(ctx, "org:1", "task_1"); !hasWorkflowCode(err, common.ResourceNotFound) {
		t.Fatalf("execution existed before approval: %v", err)
	}
	calls, err := store.ToolCalls().ListByTask(ctx, "org:1", "task_1")
	if err != nil || len(calls) != 4 {
		t.Fatalf("tool calls: %#v, %v", calls, err)
	}
	for _, call := range calls {
		if call.Status != task.ToolCallCompleted || call.ToolVersion != "v1" || call.Error != nil {
			t.Fatalf("unsafe tool audit: %#v", call)
		}
	}
	events, err := store.TaskEvents().Replay(ctx, "org:1", "task_1", 0, 100)
	if err != nil || len(events) != 14 {
		t.Fatalf("events: %#v, %v", events, err)
	}
	for index, event := range events {
		if event.Sequence != int64(index+1) {
			t.Fatalf("event sequence %d = %d", index, event.Sequence)
		}
	}
	if events[10].Type != task.EventDiagnosisCompleted || events[11].Type != task.EventIntentPrepared || events[12].Type != task.EventApprovalRequested || events[13].Type != task.EventTaskStatusChanged || item.LatestSequence != events[len(events)-1].Sequence {
		t.Fatalf("last event=%#v task sequence=%d", events[len(events)-1], item.LatestSequence)
	}
}

func TestRunIncidentRejectsUnboundedOrWriteEvidenceBeforeAuditingIt(t *testing.T) {
	for name, evidence := range map[string][]incidentport.ToolEvidence{
		"too many":   append(incidentEvidence(), incidentport.ToolEvidence{Name: "order_service.get_runtime", InputSummary: json.RawMessage(`{}`), OutputSummary: json.RawMessage(`{}`)}),
		"write tool": append(incidentEvidence()[:3], incidentport.ToolEvidence{Name: "order_service.restore_worker_concurrency", InputSummary: json.RawMessage(`{}`), OutputSummary: json.RawMessage(`{}`)}),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			store, now := openIncidentStore(t)
			seedIncidentTask(t, store, now, "task_unsafe", task.StatusCreated)
			workflow := RunIncidentWorkflow{Store: store, Toolset: incidentWorkflowToolset{evidence: evidence}, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
			if err := workflow.Run(ctx, incidentIdentity(), "task_unsafe"); err == nil {
				t.Fatal("unsafe observation unexpectedly succeeded")
			}
			item, _ := store.Tasks().Get(ctx, "org:1", "task_unsafe")
			if item.Status != task.StatusFailed || item.Error == nil || item.Error.Code != common.InvalidStateTransition {
				t.Fatalf("failed task: %#v", item)
			}
			calls, err := store.ToolCalls().ListByTask(ctx, "org:1", "task_unsafe")
			if err != nil || len(calls) != 0 {
				t.Fatalf("unsafe evidence was persisted: %#v, %v", calls, err)
			}
		})
	}
}

func TestRunIncidentCompletesPlaybookNoActionWithoutIntentApprovalOrWrite(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	seedIncidentTask(t, store, now, "task_no_action", task.StatusCreated)
	diagnosis := task.Diagnosis{PrimaryHypothesis: "slow_processing", EvidenceRefs: []string{"queue", "worker", "policy", "outcomes"}, AlternativeHypotheses: []string{"dependency_errors"}, Confidence: 0.9, CandidateAction: "no_action"}
	prepared := incidentport.PreparedRun{Status: "completed", Checkpoint: "no-action-checkpoint"}
	workflow := RunIncidentWorkflow{Store: store, Toolset: incidentWorkflowToolset{diagnosis: &diagnosis, prepared: &prepared}, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
	if err := workflow.Run(ctx, incidentIdentity(), "task_no_action"); err != nil {
		t.Fatal(err)
	}
	item, err := store.Tasks().Get(ctx, "org:1", "task_no_action")
	if err != nil || item.Status != task.StatusCompleted || item.IncidentPlan.Phase != task.PhaseNoAction || item.IncidentPlan.Intent != nil {
		t.Fatalf("no-action task=%#v err=%v", item, err)
	}
	checkpoint, err := store.TaskCheckpoints().Get(ctx, "org:1", "task_no_action")
	if err != nil || checkpoint.Phase != task.PhaseNoAction || checkpoint.OpaqueValue != "no-action-checkpoint" {
		t.Fatalf("checkpoint=%#v err=%v", checkpoint, err)
	}
	if _, err := store.Approvals().GetByTask(ctx, "org:1", "task_no_action"); !hasWorkflowCode(err, common.ResourceNotFound) {
		t.Fatalf("no-action created approval: %v", err)
	}
	if _, err := store.RemediationExecutions().GetByTask(ctx, "org:1", "task_no_action"); !hasWorkflowCode(err, common.ResourceNotFound) {
		t.Fatalf("no-action created execution: %v", err)
	}
}

func TestRunIncidentRejectsMismatchedPreparedIntentWithoutApproval(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	seedIncidentTask(t, store, now, "task_bad_prepare", task.StatusCreated)
	prepared := defaultPreparedRun()
	prepared.Intent.AfterConcurrency = 3
	workflow := RunIncidentWorkflow{Store: store, Toolset: incidentWorkflowToolset{prepared: &prepared}, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
	if err := workflow.Run(ctx, incidentIdentity(), "task_bad_prepare"); !hasWorkflowCode(err, common.SchemaValidationFailed) {
		t.Fatalf("error=%v", err)
	}
	item, _ := store.Tasks().Get(ctx, "org:1", "task_bad_prepare")
	if item.Status != task.StatusFailed || item.IncidentPlan.Intent != nil {
		t.Fatalf("unsafe prepare task=%#v", item)
	}
	if _, err := store.Approvals().GetByTask(ctx, "org:1", "task_bad_prepare"); !hasWorkflowCode(err, common.ResourceNotFound) {
		t.Fatalf("unsafe prepare created approval: %v", err)
	}
}

func TestIncidentAndAnalysisRecoveryRespectTaskKinds(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	seedIncidentTask(t, store, now, "task_recover", task.StatusPlanning)
	analysis := RunAnalysisWorkflow{Store: store, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
	if err := analysis.RecoverInterrupted(ctx); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := store.Tasks().Get(ctx, "org:1", "task_recover")
	if unchanged.Status != task.StatusPlanning {
		t.Fatalf("analysis recovery claimed incident: %#v", unchanged)
	}
	incident := RunIncidentWorkflow{Store: store, Toolset: incidentWorkflowToolset{}, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
	if err := incident.Recover(ctx, func(task.AnalysisTask) requestcontext.Context { return incidentIdentity() }); err != nil {
		t.Fatal(err)
	}
	recovered, _ := store.Tasks().Get(ctx, "org:1", "task_recover")
	if recovered.IncidentPlan == nil || recovered.IncidentPlan.Diagnosis == nil || recovered.IncidentPlan.Intent == nil || recovered.Status != task.StatusWaitingApproval {
		t.Fatalf("incident was not resumed: %#v", recovered)
	}
	if err := incident.Recover(ctx, func(task.AnalysisTask) requestcontext.Context { return incidentIdentity() }); err != nil {
		t.Fatal(err)
	}
	calls, _ := store.ToolCalls().ListByTask(ctx, "org:1", "task_recover")
	if len(calls) != 4 {
		t.Fatalf("recovery was not idempotent: %#v", calls)
	}
}

func TestIncidentRecoveryResumesPersistedDiagnosisAtPrepareWithoutRediagnosis(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	seedIncidentTask(t, store, now, "task_prepare_recovery", task.StatusCreated)
	item, _ := store.Tasks().Get(ctx, "org:1", "task_prepare_recovery")
	for _, status := range []task.Status{task.StatusPlanning, task.StatusRunningTools} {
		if err := item.Transition(status, now); err != nil {
			t.Fatal(err)
		}
		if err := store.Tasks().Update(ctx, item, item.Version-1); err != nil {
			t.Fatal(err)
		}
	}
	diagnosis := task.Diagnosis{PrimaryHypothesis: "worker_stopped", EvidenceRefs: []string{"queue", "worker", "policy", "outcomes"}, AlternativeHypotheses: []string{"slow_processing"}, Confidence: 0.99, CandidateAction: "restore_worker_concurrency"}
	if err := item.RecordDiagnosis(diagnosis, now); err != nil {
		t.Fatal(err)
	}
	checkpoint, _ := store.TaskCheckpoints().Get(ctx, "org:1", item.ID)
	if err := checkpoint.Replace(task.PhasePrepare, checkpoint.OpaqueValue, now); err != nil {
		t.Fatal(err)
	}
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Tasks().Update(ctx, item, item.Version-1); err != nil {
			return err
		}
		return tx.TaskCheckpoints().Update(ctx, checkpoint, checkpoint.Version-1)
	}); err != nil {
		t.Fatal(err)
	}
	workflow := RunIncidentWorkflow{Store: store, Toolset: incidentWorkflowToolset{}, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
	if err := workflow.Recover(ctx, func(task.AnalysisTask) requestcontext.Context { return incidentIdentity() }); err != nil {
		t.Fatal(err)
	}
	recovered, _ := store.Tasks().Get(ctx, "org:1", item.ID)
	if recovered.Status != task.StatusWaitingApproval || recovered.IncidentPlan.Intent == nil {
		t.Fatalf("recovered=%#v", recovered)
	}
	calls, err := store.ToolCalls().ListByTask(ctx, "org:1", item.ID)
	if err != nil || len(calls) != 0 {
		t.Fatalf("diagnosis was replayed: %#v err=%v", calls, err)
	}
}

func TestRunIncidentPreservesWrappedDomainFailure(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	seedIncidentTask(t, store, now, "task_failed", task.StatusCreated)
	cause := fmt.Errorf("adapter: %w", common.NewError(common.DependencyUnavailable, "diagnostic dependency unavailable", true))
	workflow := RunIncidentWorkflow{Store: store, Toolset: incidentWorkflowToolset{err: cause}, IDs: &sequenceIDs{}, Clock: fixedClock{now: now}}
	if err := workflow.Run(ctx, incidentIdentity(), "task_failed"); !errors.Is(err, cause) {
		t.Fatalf("error = %v", err)
	}
	failed, _ := store.Tasks().Get(ctx, "org:1", "task_failed")
	if failed.Error == nil || failed.Error.Code != common.DependencyUnavailable {
		t.Fatalf("domain error was erased: %#v", failed.Error)
	}
}

type incidentWorkflowToolset struct {
	evidence   []incidentport.ToolEvidence
	diagnosis  *task.Diagnosis
	prepared   *incidentport.PreparedRun
	err        error
	prepareErr error
}

func (incidentWorkflowToolset) ResolveAndStart(context.Context, requestcontext.Context, string, string, map[string]string) (incidentport.ResolvedRun, error) {
	return incidentport.ResolvedRun{}, errors.New("not used")
}

func (f incidentWorkflowToolset) Observe(context.Context, requestcontext.Context, string) (incidentport.Observation, error) {
	if f.err != nil {
		return incidentport.Observation{}, f.err
	}
	evidence := f.evidence
	if evidence == nil {
		evidence = incidentEvidence()
	}
	diagnosis := task.Diagnosis{PrimaryHypothesis: "worker_stopped", EvidenceRefs: []string{"queue", "worker", "policy", "outcomes"}, AlternativeHypotheses: []string{"slow_processing", "dependency_errors"}, Confidence: 0.99, CandidateAction: "restore_worker_concurrency"}
	if f.diagnosis != nil {
		diagnosis = *f.diagnosis
	}
	return incidentport.Observation{Diagnosis: diagnosis, Evidence: evidence}, nil
}

func (f incidentWorkflowToolset) Prepare(context.Context, requestcontext.Context, string, task.Diagnosis) (incidentport.PreparedRun, error) {
	if f.prepareErr != nil {
		return incidentport.PreparedRun{}, f.prepareErr
	}
	if f.prepared != nil {
		return *f.prepared, nil
	}
	return defaultPreparedRun(), nil
}

func defaultPreparedRun() incidentport.PreparedRun {
	return incidentport.PreparedRun{Status: "needs_approval", Checkpoint: "prepared-checkpoint", Intent: &incidentport.PreparedIntent{CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: "order-demo", InstanceEpoch: "epoch-1", ExpectedVersion: 2, ObservedAt: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC), PolicyDigest: strings.Repeat("b", 64), PlaybookDigest: strings.Repeat("a", 64), BeforeConcurrency: 0, AfterConcurrency: 2, RiskSummary: "bounded restore"}}
}

func hasWorkflowCode(err error, code common.ErrorCode) bool {
	var domainErr *common.DomainError
	return errors.As(err, &domainErr) && domainErr.Code == code
}

func incidentEvidence() []incidentport.ToolEvidence {
	names := []string{"order_service.get_queue_snapshot", "order_service.get_worker_state", "order_service.get_worker_policy", "order_service.get_recent_outcomes"}
	result := make([]incidentport.ToolEvidence, 0, len(names))
	for _, name := range names {
		result = append(result, incidentport.ToolEvidence{Name: name, InputSummary: json.RawMessage(`{}`), OutputSummary: json.RawMessage(`{"bounded":true}`), DurationMS: 3})
	}
	return result
}

func openIncidentStore(t *testing.T) (*storage.Store, time.Time) {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "ai-core.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
}

func seedIncidentTask(t *testing.T, store repositories.ApplicationStore, now time.Time, taskID string, status task.Status) {
	t.Helper()
	ctx := context.Background()
	incidentSession, _ := session.NewIncident("session_"+taskID, "org:1", "1", "Queue backlog", "system:grafana", now)
	message, _ := session.NewMessage("message_"+taskID, "org:1", incidentSession.ID, taskID, session.RoleTrigger, "OrderQueueBacklog firing", now)
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	plan := task.IncidentPlan{SourceID: "demo-grafana", AlertName: "OrderQueueBacklog", AlertFingerprint: taskID, ServiceRef: "order-demo", Labels: map[string]string{"service_ref": "order-demo"}, Mapping: task.PinnedRef{ID: "mapping", Digest: digest}, Playbook: task.PinnedRef{ID: "playbook", Version: "1.0.0", Digest: digest}, AssetRefs: []task.AssetRef{{Kind: "knowledge", ID: "knowledge", Version: "1.0.0", Digest: digest}, {Kind: "skill", ID: "skill", Version: "1.0.0", Digest: digest}, {Kind: "playbook", ID: "playbook", Version: "1.0.0", Digest: digest}}, Phase: task.PhaseNeedsAgent}
	item, err := task.NewIncident(taskID, "org:1", incidentSession.ID, message.ID, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if status == task.StatusPlanning {
		if err := item.Transition(task.StatusPlanning, now); err != nil {
			t.Fatal(err)
		}
	}
	checkpoint, _ := task.NewCheckpoint(taskID, "org:1", task.PhaseNeedsAgent, "signed-checkpoint", now)
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Sessions().Create(ctx, incidentSession); err != nil {
			return err
		}
		if err := tx.Messages().Append(ctx, message); err != nil {
			return err
		}
		if err := tx.Tasks().Create(ctx, item); err != nil {
			return err
		}
		return tx.TaskCheckpoints().Create(ctx, checkpoint)
	}); err != nil {
		t.Fatal(err)
	}
}

func incidentIdentity() requestcontext.Context {
	return requestcontext.Context{TenantID: "org:1", OrgID: "1", UserID: "system:grafana", RequestID: "request_1", TraceID: "trace_1"}
}
