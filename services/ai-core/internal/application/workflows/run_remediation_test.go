package workflows

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	approvalevidence "mini-torchbearing.local/packages/approval-evidence-go"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/remediation"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	incidentport "mini-torchbearing.local/services/ai-core/internal/ports/incident"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

func TestRunRemediationCompletesOneWriteAndThreePartVerification(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	ids := &sequenceIDs{}
	seedApprovedIncident(t, store, ids, now, "task_execute")
	tools := &remediationTools{now: now}
	workflow := remediationWorkflow(t, store, ids, now, tools)

	if err := workflow.RunApproved(ctx, remediationIdentity(), "task_execute"); err != nil {
		t.Fatal(err)
	}
	item, _ := store.Tasks().Get(ctx, "org:1", "task_execute")
	if item.Status != task.StatusCompleted || item.IncidentPlan.Phase != task.PhaseCompleted {
		t.Fatalf("task=%#v", item)
	}
	execution, err := store.RemediationExecutions().GetByTask(ctx, "org:1", item.ID)
	if err != nil || execution.State != remediation.ExecutionApplied || execution.BeforeVersion == nil || *execution.BeforeVersion != 2 || execution.AfterVersion == nil || *execution.AfterVersion != 3 {
		t.Fatalf("execution=%#v err=%v", execution, err)
	}
	if tools.restoreCalls != 1 || tools.operationCalls != 0 || tools.runtimeCalls != 1 || tools.workerCalls != 1 || tools.metricsCalls != 2 || tools.probeCalls != 1 {
		t.Fatalf("calls=%#v", tools)
	}
	if tools.lastEvidence == "" {
		t.Fatal("approval evidence was not sent to the typed write")
	}
	codec, _ := approvalevidence.New([]byte("0123456789abcdef0123456789abcdef"))
	if _, err := codec.Verify(tools.lastEvidence, approvalevidence.ExpectedScope{TenantID: "org:1", OrgID: "1", ApprovalID: execution.ApprovalID, IntentDigest: execution.IntentDigest, CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: "order-demo", InstanceEpoch: execution.InstanceEpoch, ExpectedVersion: int(execution.ExpectedVersion), OperationID: execution.OperationID}, now); err != nil {
		t.Fatalf("approval evidence scope=%v", err)
	}
	calls, err := store.ToolCalls().ListByTask(ctx, "org:1", item.ID)
	if err != nil || len(calls) != 10 {
		t.Fatalf("tool calls=%d err=%v", len(calls), err)
	}
	for _, call := range calls {
		if strings.Contains(string(call.InputSummary), tools.lastEvidence) || strings.Contains(string(call.OutputSummary), tools.lastEvidence) {
			t.Fatalf("approval evidence leaked into durable ToolCall: %#v", call)
		}
	}
	audits, err := store.AuditRecords().ListByTask(ctx, "org:1", item.ID)
	if err != nil || len(audits) != 3 || audits[0].Outcome != remediation.AuditAccepted || audits[1].Action != remediation.AuditRemediationExecute || audits[1].Outcome != remediation.AuditSucceeded || audits[2].Action != remediation.AuditRemediationVerify || audits[2].Outcome != remediation.AuditSucceeded {
		t.Fatalf("audits=%#v err=%v", audits, err)
	}
	events, _ := store.TaskEvents().Replay(ctx, "org:1", item.ID, 0, 200)
	requireEventOrder(t, events, task.EventRemediationStarted, task.EventRemediationReconciled, task.EventVerificationRuntime, task.EventVerificationMetrics, task.EventVerificationBusiness, task.EventTaskCompleted)
	if err := workflow.RunApproved(ctx, remediationIdentity(), item.ID); err != nil {
		t.Fatal(err)
	}
	if tools.restoreCalls != 1 {
		t.Fatalf("terminal replay repeated write: %d", tools.restoreCalls)
	}
}

func TestRunRemediationWriteTimeoutReconcilesWithoutSecondWrite(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	ids := &sequenceIDs{}
	seedApprovedIncident(t, store, ids, now, "task_timeout")
	tools := &remediationTools{now: now, restoreErr: common.NewError(common.ToolTimeout, "write response timed out", true), operationExists: true}
	workflow := remediationWorkflow(t, store, ids, now, tools)

	if err := workflow.RunApproved(ctx, remediationIdentity(), "task_timeout"); err != nil {
		t.Fatal(err)
	}
	execution, _ := store.RemediationExecutions().GetByTask(ctx, "org:1", "task_timeout")
	if execution.State != remediation.ExecutionAlreadyApplied || execution.Version != 3 || tools.restoreCalls != 1 || tools.operationCalls != 1 {
		t.Fatalf("execution=%#v restore=%d reconcile=%d", execution, tools.restoreCalls, tools.operationCalls)
	}
	calls, _ := store.ToolCalls().ListByTask(ctx, "org:1", "task_timeout")
	failedWrites := 0
	for _, call := range calls {
		if call.ToolName == "order_service.restore_worker_concurrency" && call.Status == task.ToolCallFailed && call.Error != nil && call.Error.Code == common.ToolTimeout {
			failedWrites++
		}
	}
	if failedWrites != 1 {
		t.Fatalf("failed write audit count=%d calls=%#v", failedWrites, calls)
	}
}

func TestRunRemediationRestartedStartedExecutionUsesOnlyReadReconcile(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	ids := &sequenceIDs{}
	seedApprovedIncident(t, store, ids, now, "task_restart")
	item, _ := store.Tasks().Get(ctx, "org:1", "task_restart")
	approval, _ := store.Approvals().GetByTask(ctx, "org:1", item.ID)
	execution, _ := remediation.NewExecution("operation_before_crash", item.TenantID, "1", item.ID, approval.ID, item.IncidentPlan.Intent.Digest, item.IncidentPlan.Intent.InstanceEpoch, item.IncidentPlan.Intent.ExpectedVersion, now)
	checkpoint, _ := store.TaskCheckpoints().Get(ctx, item.TenantID, item.ID)
	_ = item.Transition(task.StatusExecuting, now)
	_ = checkpoint.Replace(task.PhaseExecute, checkpoint.OpaqueValue, now)
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.RemediationExecutions().Create(ctx, execution); err != nil {
			return err
		}
		if err := tx.Tasks().Update(ctx, item, item.Version-1); err != nil {
			return err
		}
		return tx.TaskCheckpoints().Update(ctx, checkpoint, checkpoint.Version-1)
	}); err != nil {
		t.Fatal(err)
	}
	tools := &remediationTools{now: now, operationExists: true, operationID: execution.OperationID, approvalID: execution.ApprovalID, intentDigest: execution.IntentDigest}
	workflow := remediationWorkflow(t, store, ids, now, tools)

	if err := workflow.RunApproved(ctx, remediationIdentity(), item.ID); err != nil {
		t.Fatal(err)
	}
	loaded, _ := store.RemediationExecutions().GetByTask(ctx, item.TenantID, item.ID)
	if tools.restoreCalls != 0 || tools.operationCalls != 1 || loaded.State != remediation.ExecutionAlreadyApplied || loaded.Version != 3 {
		t.Fatalf("write=%d reconcile=%d execution=%#v", tools.restoreCalls, tools.operationCalls, loaded)
	}
}

func TestRunRemediationRecoversUnknownReconcileAndBusinessCheckpointWithoutWrite(t *testing.T) {
	for _, test := range []struct {
		name       string
		checkpoint task.IncidentPhase
		state      remediation.ExecutionState
		wantOp     int
		wantMetric int
	}{
		{"unknown reconcile", task.PhaseVerifyRuntime, remediation.ExecutionUnknown, 1, 2},
		{"business checkpoint", task.PhaseVerifyBusiness, remediation.ExecutionApplied, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			store, now := openIncidentStore(t)
			ids := &sequenceIDs{}
			seedApprovedIncident(t, store, ids, now, "task_"+strings.ReplaceAll(test.name, " ", "_"))
			item, _ := store.Tasks().Get(ctx, "org:1", "task_"+strings.ReplaceAll(test.name, " ", "_"))
			expectedTaskVersion := item.Version
			approval, _ := store.Approvals().GetByTask(ctx, item.TenantID, item.ID)
			execution, _ := remediation.NewExecution("operation_recovery", item.TenantID, "1", item.ID, approval.ID, item.IncidentPlan.Intent.Digest, "epoch-1", 2, now)
			_ = execution.RecordReceipt(2, 3, false, now)
			if test.state == remediation.ExecutionUnknown {
				execution, _ = remediation.NewExecution("operation_recovery", item.TenantID, "1", item.ID, approval.ID, item.IncidentPlan.Intent.Digest, "epoch-1", 2, now)
				_ = execution.MarkUnknown(now)
			}
			_ = item.Transition(task.StatusExecuting, now)
			_ = item.Transition(task.StatusReconciling, now)
			if test.checkpoint == task.PhaseVerifyBusiness {
				_ = item.Transition(task.StatusValidating, now)
			}
			item.Version = expectedTaskVersion + 1
			checkpoint, _ := store.TaskCheckpoints().Get(ctx, item.TenantID, item.ID)
			_ = checkpoint.Replace(test.checkpoint, checkpoint.OpaqueValue, now)
			if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
				if err := tx.RemediationExecutions().Create(ctx, remediation.Execution{OperationID: execution.OperationID, TenantID: execution.TenantID, OrgID: execution.OrgID, TaskID: execution.TaskID, ApprovalID: execution.ApprovalID, IntentDigest: execution.IntentDigest, InstanceEpoch: execution.InstanceEpoch, ExpectedVersion: execution.ExpectedVersion, State: remediation.ExecutionStarted, StartedAt: execution.StartedAt, Version: 1}); err != nil {
					return err
				}
				if execution.State != remediation.ExecutionStarted {
					if err := tx.RemediationExecutions().Update(ctx, execution, 1); err != nil {
						return err
					}
				}
				if err := tx.Tasks().Update(ctx, item, expectedTaskVersion); err != nil {
					return err
				}
				return tx.TaskCheckpoints().Update(ctx, checkpoint, checkpoint.Version-1)
			}); err != nil {
				t.Fatal(err)
			}
			tools := &remediationTools{now: now, operationExists: true, operationID: execution.OperationID, approvalID: execution.ApprovalID, intentDigest: execution.IntentDigest}
			workflow := remediationWorkflow(t, store, ids, now, tools)
			if err := workflow.RunApproved(ctx, remediationIdentity(), item.ID); err != nil {
				t.Fatal(err)
			}
			if tools.restoreCalls != 0 || tools.operationCalls != test.wantOp || tools.metricsCalls != test.wantMetric || tools.probeCalls != 1 {
				t.Fatalf("calls=%#v", tools)
			}
		})
	}
}

func TestRunRemediationFailsClosedWhenUnknownReceiptIsUnavailable(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	ids := &sequenceIDs{}
	seedApprovedIncident(t, store, ids, now, "task_unknown")
	tools := &remediationTools{now: now, restoreErr: common.NewError(common.ToolTimeout, "timeout", true), operationErr: common.NewError(common.ResourceNotFound, "operation missing", false)}
	workflow := remediationWorkflow(t, store, ids, now, tools)

	if err := workflow.RunApproved(ctx, remediationIdentity(), "task_unknown"); err == nil {
		t.Fatal("expected unknown receipt failure")
	}
	item, _ := store.Tasks().Get(ctx, "org:1", "task_unknown")
	execution, _ := store.RemediationExecutions().GetByTask(ctx, "org:1", item.ID)
	if item.Status != task.StatusFailed || execution.State != remediation.ExecutionUnknown || tools.restoreCalls != 1 || tools.operationCalls != 1 {
		t.Fatalf("task=%#v execution=%#v calls=%#v", item, execution, tools)
	}
}

func TestRunRemediationVerificationFailureNeverReplaysWrite(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	ids := &sequenceIDs{}
	seedApprovedIncident(t, store, ids, now, "task_bad_metrics")
	tools := &remediationTools{now: now, unhealthyMetrics: true}
	workflow := remediationWorkflow(t, store, ids, now, tools)

	if err := workflow.RunApproved(ctx, remediationIdentity(), "task_bad_metrics"); err == nil {
		t.Fatal("expected metric verification failure")
	}
	item, _ := store.Tasks().Get(ctx, "org:1", "task_bad_metrics")
	if item.Status != task.StatusFailed || tools.restoreCalls != 1 || tools.probeCalls != 0 {
		t.Fatalf("task=%#v calls=%#v", item, tools)
	}
	if err := workflow.RunApproved(ctx, remediationIdentity(), item.ID); err != nil {
		t.Fatal(err)
	}
	if tools.restoreCalls != 1 {
		t.Fatalf("failed replay repeated write: %d", tools.restoreCalls)
	}
	audits, _ := store.AuditRecords().ListByTask(ctx, "org:1", item.ID)
	if audits[len(audits)-1].Action != remediation.AuditRemediationVerify || audits[len(audits)-1].Outcome != remediation.AuditFailed {
		t.Fatalf("audits=%#v", audits)
	}
}

func TestRunRemediationRejectsMissingPermissionBeforeAnyWrite(t *testing.T) {
	store, now := openIncidentStore(t)
	ids := &sequenceIDs{}
	seedApprovedIncident(t, store, ids, now, "task_permission")
	tools := &remediationTools{now: now}
	workflow := remediationWorkflow(t, store, ids, now, tools)
	identity := remediationIdentity()
	identity.Permissions = []string{"incidents:diagnose"}
	if err := workflow.RunApproved(context.Background(), identity, "task_permission"); !hasWorkflowCode(err, common.PermissionDenied) {
		t.Fatalf("error=%v", err)
	}
	if tools.restoreCalls != 0 {
		t.Fatal("permission failure reached write tool")
	}
	if _, err := store.RemediationExecutions().GetByTask(context.Background(), "org:1", "task_permission"); !hasWorkflowCode(err, common.ResourceNotFound) {
		t.Fatalf("execution=%v", err)
	}
}

type remediationTools struct {
	now                                                     time.Time
	restoreCalls, operationCalls, runtimeCalls, workerCalls int
	metricsCalls, probeCalls                                int
	restoreErr, operationErr                                error
	operationExists, unhealthyMetrics                       bool
	operationID, approvalID, intentDigest, lastEvidence     string
}

func (f *remediationTools) RestoreWorkerConcurrency(_ context.Context, _ requestcontext.Context, request incidentport.RestoreRequest) (incidentport.OperationReceipt, incidentport.ToolEvidence, error) {
	f.restoreCalls++
	f.operationID, f.approvalID, f.intentDigest, f.lastEvidence = request.OperationID, request.ApprovalID, request.IntentDigest, request.ApprovalEvidence
	receipt := f.receipt(request.OperationID, request.ApprovalID, request.IntentDigest)
	evidence := toolEvidence("order_service.restore_worker_concurrency", map[string]any{"operationId": request.OperationID, "approvalEvidencePresent": true}, receipt)
	return receipt, evidence, f.restoreErr
}

func (f *remediationTools) GetOperation(_ context.Context, _ requestcontext.Context, operationID string) (incidentport.OperationReceipt, incidentport.ToolEvidence, error) {
	f.operationCalls++
	evidence := toolEvidence("order_service.get_operation", map[string]any{"operationId": operationID}, map[string]any{"found": f.operationExists})
	if f.operationErr != nil {
		return incidentport.OperationReceipt{}, evidence, f.operationErr
	}
	if !f.operationExists {
		return incidentport.OperationReceipt{}, evidence, common.NewError(common.ResourceNotFound, "operation missing", false)
	}
	return f.receipt(operationID, f.approvalID, f.intentDigest), evidence, nil
}

func (f *remediationTools) GetRuntime(context.Context, requestcontext.Context) (incidentport.RuntimeState, incidentport.ToolEvidence, error) {
	f.runtimeCalls++
	value := incidentport.RuntimeState{ServiceRef: "order-demo", InstanceEpoch: "epoch-1", SupervisorStatus: "running", StartedAt: f.now.Add(-time.Minute)}
	return value, toolEvidence("order_service.get_runtime", map[string]any{}, value), nil
}

func (f *remediationTools) GetWorker(context.Context, requestcontext.Context) (incidentport.WorkerState, incidentport.ToolEvidence, error) {
	f.workerCalls++
	value := incidentport.WorkerState{ServiceRef: "order-demo", InstanceEpoch: "epoch-1", ConfiguredConcurrency: 2, EffectiveConcurrency: 2, ActiveWorkers: 2, Version: 3, ObservedAt: f.now.Add(time.Second)}
	return value, toolEvidence("order_service.get_worker_state", map[string]any{}, value), nil
}

func (f *remediationTools) GetRecoveryMetrics(context.Context, requestcontext.Context) (incidentport.RecoveryMetrics, incidentport.ToolEvidence, error) {
	f.metricsCalls++
	value := incidentport.RecoveryMetrics{WindowSeconds: 30, AcceptedDelta: 2, CompletedDelta: 4, QueueDepth: 1, OldestAgeSeconds: .2, ObservedAt: f.now.Add(time.Duration(f.metricsCalls) * time.Second)}
	if f.unhealthyMetrics {
		value.CompletedDelta = 0
		value.QueueDepth = 20
	}
	return value, toolEvidence("order_service.get_recovery_metrics", map[string]any{}, value), nil
}

func (f *remediationTools) RunBusinessProbe(_ context.Context, _ requestcontext.Context, probeID string) (incidentport.BusinessProbe, incidentport.ToolEvidence, error) {
	f.probeCalls++
	value := incidentport.BusinessProbe{ProbeID: probeID, Result: "completed", DurationMS: 200, CompletedAt: f.now.Add(4 * time.Second)}
	return value, toolEvidence("order_service.run_business_probe", map[string]any{"probeId": probeID}, value), nil
}

func (f *remediationTools) receipt(operationID, approvalID, digest string) incidentport.OperationReceipt {
	return incidentport.OperationReceipt{OperationID: operationID, InstanceEpoch: "epoch-1", IntentDigest: digest, ApprovalID: approvalID, BeforeVersion: 2, AfterVersion: 3, BeforeConcurrency: 0, AfterConcurrency: 2, ExecutedAt: f.now.Add(time.Second)}
}

func toolEvidence(name string, input, output any) incidentport.ToolEvidence {
	in, _ := json.Marshal(input)
	out, _ := json.Marshal(output)
	return incidentport.ToolEvidence{Name: name, InputSummary: in, OutputSummary: out, DurationMS: 3}
}

func remediationWorkflow(t *testing.T, store repositories.ApplicationStore, ids *sequenceIDs, now time.Time, tools *remediationTools) RunRemediationWorkflow {
	t.Helper()
	codec, err := approvalevidence.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return RunRemediationWorkflow{Store: store, Toolset: tools, Evidence: codec, IDs: ids, Clock: fixedClock{now: now}, VerificationInterval: -1}
}

func seedApprovedIncident(t *testing.T, store repositories.ApplicationStore, ids *sequenceIDs, now time.Time, taskID string) {
	t.Helper()
	seedIncidentTask(t, store, now, taskID, task.StatusCreated)
	diagnostic := RunIncidentWorkflow{Store: store, Toolset: incidentWorkflowToolset{}, IDs: ids, Clock: fixedClock{now: now}}
	if err := diagnostic.Run(context.Background(), incidentIdentity(), taskID); err != nil {
		t.Fatal(err)
	}
	approval, err := store.Approvals().GetByTask(context.Background(), "org:1", taskID)
	if err != nil {
		t.Fatal(err)
	}
	if err := approval.Decide(remediation.DecisionApprove, "admin:1", "approved in test", now); err != nil {
		t.Fatal(err)
	}
	if err := store.Approvals().Update(context.Background(), approval, 1); err != nil {
		t.Fatal(err)
	}
}

func remediationIdentity() requestcontext.Context {
	return requestcontext.Context{TenantID: "org:1", OrgID: "1", UserID: "admin:1", Roles: []string{"Admin"}, Permissions: []string{"incidents:remediate"}, RequestID: "request-remediate", TraceID: "trace-remediate"}
}

func requireEventOrder(t *testing.T, events []task.TaskEvent, want ...task.EventType) {
	t.Helper()
	index := 0
	for _, event := range events {
		if index < len(want) && event.Type == want[index] {
			index++
		}
	}
	if index != len(want) {
		t.Fatalf("missing ordered event %q after index %d in %#v", want[index], index, events)
	}
}

func TestRunRemediationRejectsTamperedReceiptThenUsesAuthoritativeOperation(t *testing.T) {
	ctx := context.Background()
	store, now := openIncidentStore(t)
	ids := &sequenceIDs{}
	seedApprovedIncident(t, store, ids, now, "task_receipt")
	tools := &tamperedReceiptTools{remediationTools: remediationTools{now: now, operationExists: true}}
	workflow := remediationWorkflow(t, store, ids, now, &tools.remediationTools)
	// Wrap just the typed toolset so the initial response is malformed while
	// the read-only operation lookup returns the durable authoritative receipt.
	workflow.Toolset = tools
	if err := workflow.RunApproved(ctx, remediationIdentity(), "task_receipt"); err != nil {
		t.Fatal(err)
	}
	if tools.restoreCalls != 1 || tools.operationCalls != 1 {
		t.Fatalf("calls=%#v", tools)
	}
}

type tamperedReceiptTools struct{ remediationTools }

func (f *tamperedReceiptTools) RestoreWorkerConcurrency(ctx context.Context, identity requestcontext.Context, request incidentport.RestoreRequest) (incidentport.OperationReceipt, incidentport.ToolEvidence, error) {
	receipt, evidence, err := f.remediationTools.RestoreWorkerConcurrency(ctx, identity, request)
	receipt.AfterConcurrency = 3
	return receipt, evidence, err
}

var _ incidentport.RemediationToolset = (*remediationTools)(nil)
var _ incidentport.RemediationToolset = (*tamperedReceiptTools)(nil)
