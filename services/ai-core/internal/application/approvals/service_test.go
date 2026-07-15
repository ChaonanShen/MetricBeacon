package approvals

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	storage "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/storage/sqlite"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/remediation"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

func TestApprovalServiceEnforcesAdminScopeIdempotencyAndSchedulesApprovedTask(t *testing.T) {
	ctx := context.Background()
	store, now := approvalStore(t)
	item, approval := seedPendingApproval(t, ctx, store, "approved", now)
	workflow := &approvedWorkflowStub{calls: make(chan requestcontext.Context, 2)}
	service := New(store, nil, workflow, &approvalIDs{}, approvalClock{now})

	if _, err := service.Get(ctx, approvalIdentity("other-org", "Viewer"), item.ID); !approvalCode(err, common.ResourceNotFound) {
		t.Fatalf("cross-org get error=%v", err)
	}
	input := approvalInput(item, approval, "approve", "decision-key")
	if _, err := service.Decide(ctx, approvalIdentity("1", "Viewer"), input); !approvalCode(err, common.PermissionDenied) {
		t.Fatalf("Viewer decision error=%v", err)
	}
	tampered := input
	tampered.IdempotencyKey = "tampered-key"
	tampered.IntentDigest = "sha256:" + strings.Repeat("c", 64)
	if _, err := service.Decide(ctx, approvalIdentity("1", "Admin"), tampered); !approvalCode(err, common.ResourceConflict) {
		t.Fatalf("tampered Intent error=%v", err)
	}
	result, err := service.Decide(ctx, approvalIdentity("1", "Admin"), input)
	if err != nil || result.Status != remediation.ApprovalApproved || result.Version != 2 {
		t.Fatalf("decision=%#v err=%v", result, err)
	}
	select {
	case executionIdentity := <-workflow.calls:
		if len(executionIdentity.Permissions) != 1 || executionIdentity.Permissions[0] != "incidents:remediate" || executionIdentity.UserID != "admin:1" {
			t.Fatalf("execution identity=%#v", executionIdentity)
		}
	case <-time.After(time.Second):
		t.Fatal("approved workflow was not scheduled")
	}
	replayed, err := service.Decide(ctx, approvalIdentity("1", "Admin"), input)
	if err != nil || replayed.ID != result.ID || replayed.Version != 2 {
		t.Fatalf("idempotent replay=%#v err=%v", replayed, err)
	}
	select {
	case <-workflow.calls:
		t.Fatal("idempotent replay scheduled duplicate execution")
	case <-time.After(20 * time.Millisecond):
	}
	audits, err := store.AuditRecords().ListByTask(ctx, "org:1", item.ID)
	if err != nil || len(audits) != 1 || audits[0].Outcome != remediation.AuditAccepted {
		t.Fatalf("audits=%#v err=%v", audits, err)
	}
	events, err := store.TaskEvents().Replay(ctx, "org:1", item.ID, 0, 10)
	if err != nil || len(events) != 2 || events[0].Type != task.EventApprovalDecided || events[1].Type != task.EventAuditRecorded {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	loadedTask, _ := store.Tasks().Get(ctx, "org:1", item.ID)
	if loadedTask.Status != task.StatusWaitingApproval {
		t.Fatalf("HTTP approval executed or moved Task prematurely: %#v", loadedTask)
	}
}

func TestApprovalServiceConcurrentCASAllowsOneDecision(t *testing.T) {
	ctx := context.Background()
	store, now := approvalStore(t)
	item, approval := seedPendingApproval(t, ctx, store, "concurrent", now)
	workflow := &approvedWorkflowStub{calls: make(chan requestcontext.Context, 2)}
	service := New(store, nil, workflow, &approvalIDs{}, approvalClock{now})
	start := make(chan struct{})
	errs := make(chan error, 2)
	var group sync.WaitGroup
	for index := 0; index < 2; index++ {
		index := index
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			input := approvalInput(item, approval, "approve", fmt.Sprintf("decision-%d", index))
			_, err := service.Decide(ctx, approvalIdentity("1", "Admin"), input)
			errs <- err
		}()
	}
	close(start)
	group.Wait()
	close(errs)
	succeeded, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			succeeded++
		} else if approvalCode(err, common.ResourceConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected error=%v", err)
		}
	}
	if succeeded != 1 || conflicts != 1 {
		t.Fatalf("succeeded=%d conflicts=%d", succeeded, conflicts)
	}
}

func TestApprovalServiceRejectsOrExpiresWithoutScheduling(t *testing.T) {
	for _, test := range []struct {
		name     string
		decision string
		advance  time.Duration
		status   remediation.ApprovalStatus
	}{
		{"rejected", "reject", 0, remediation.ApprovalRejected},
		{"expired cannot approve", "approve", remediation.DefaultApprovalTTL, remediation.ApprovalExpired},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			store, now := approvalStore(t)
			item, approval := seedPendingApproval(t, ctx, store, test.name, now)
			workflow := &approvedWorkflowStub{calls: make(chan requestcontext.Context, 1)}
			service := New(store, nil, workflow, &approvalIDs{}, approvalClock{now.Add(test.advance)})
			result, err := service.Decide(ctx, approvalIdentity("1", "Admin"), approvalInput(item, approval, test.decision, "decision-key"))
			if err != nil || result.Status != test.status {
				t.Fatalf("decision=%#v err=%v", result, err)
			}
			loaded, _ := store.Tasks().Get(ctx, "org:1", item.ID)
			if loaded.Status != task.StatusCancelled {
				t.Fatalf("Task=%#v", loaded)
			}
			select {
			case <-workflow.calls:
				t.Fatal("non-approved decision scheduled execution")
			case <-time.After(20 * time.Millisecond):
			}
		})
	}
}

type approvedWorkflowStub struct{ calls chan requestcontext.Context }

func (w *approvedWorkflowStub) RunApproved(_ context.Context, identity requestcontext.Context, _ string) error {
	w.calls <- identity
	return nil
}

type approvalClock struct{ now time.Time }

func (c approvalClock) Now() time.Time { return c.now }

type approvalIDs struct{ next atomic.Int64 }

func (g *approvalIDs) NewID(kind string) string { return fmt.Sprintf("%s-%d", kind, g.next.Add(1)) }

func approvalIdentity(orgID, role string) requestcontext.Context {
	return requestcontext.Context{TenantID: "org:1", OrgID: orgID, UserID: "admin:1", Roles: []string{role}, RequestID: "request-1", TraceID: "trace-1"}
}

func approvalInput(item task.AnalysisTask, approval remediation.Approval, decision, key string) DecisionInput {
	return DecisionInput{TaskID: item.ID, Decision: decision, Reason: "reviewed bounded diff", IntentDigest: approval.IntentDigest, IdempotencyKey: key, ExpectedTaskVersion: item.Version, ExpectedApprovalVersion: approval.Version}
}

func approvalStore(t *testing.T) (*storage.Store, time.Time) {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "approval.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
}

func seedPendingApproval(t *testing.T, ctx context.Context, store repositories.ApplicationStore, suffix string, now time.Time) (task.AnalysisTask, remediation.Approval) {
	t.Helper()
	suffix = strings.NewReplacer(" ", "_", ":", "_").Replace(suffix)
	sessionID, taskID, messageID := "session_"+suffix, "task_"+suffix, "message_"+suffix
	incidentSession, _ := session.NewIncident(sessionID, "org:1", "1", "Order backlog", "system:grafana", now)
	message, _ := session.NewMessage(messageID, "org:1", sessionID, taskID, session.RoleTrigger, "OrderQueueBacklog firing", now)
	digest := "sha256:" + strings.Repeat("a", 64)
	plan := task.IncidentPlan{SourceID: "demo-grafana", AlertName: "OrderQueueBacklog", AlertFingerprint: suffix, ServiceRef: "order-demo", Labels: map[string]string{"service_ref": "order-demo"}, Mapping: task.PinnedRef{ID: "mapping", Digest: digest}, Playbook: task.PinnedRef{ID: "playbook", Version: "1", Digest: digest}, AssetRefs: []task.AssetRef{{Kind: "knowledge", ID: "knowledge", Version: "1", Digest: digest}, {Kind: "skill", ID: "skill", Version: "1", Digest: digest}, {Kind: "playbook", ID: "playbook", Version: "1", Digest: digest}}, Phase: task.PhaseNeedsAgent}
	item, _ := task.NewIncident(taskID, "org:1", sessionID, messageID, plan, now)
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Sessions().Create(ctx, incidentSession); err != nil {
			return err
		}
		if err := tx.Messages().Append(ctx, message); err != nil {
			return err
		}
		return tx.Tasks().Create(ctx, item)
	}); err != nil {
		t.Fatal(err)
	}
	for _, status := range []task.Status{task.StatusPlanning, task.StatusRunningTools} {
		_ = item.Transition(status, now)
		if err := store.Tasks().Update(ctx, item, item.Version-1); err != nil {
			t.Fatal(err)
		}
	}
	diagnosis := task.Diagnosis{PrimaryHypothesis: "worker_stopped", EvidenceRefs: []string{"worker", "policy"}, Confidence: 0.99, CandidateAction: "restore_worker_concurrency"}
	_ = item.RecordDiagnosis(diagnosis, now)
	if err := store.Tasks().Update(ctx, item, item.Version-1); err != nil {
		t.Fatal(err)
	}
	intent := remediation.Intent{ID: "intent_" + suffix, Digest: "sha256:" + strings.Repeat("b", 64), CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: "order-demo", InstanceEpoch: "epoch-1", ExpectedVersion: 2, BeforeConcurrency: 0, AfterConcurrency: 2, Risk: "low", CreatedAt: now}
	_ = item.RecordIntent(intent, now)
	_ = item.Transition(task.StatusWaitingApproval, now)
	record, _ := remediation.NewIntentRecord("org:1", "1", item.ID, intent)
	approval, _ := remediation.NewApproval("approval_"+suffix, "org:1", "1", item.ID, intent.ID, intent.Digest, now)
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Tasks().Update(ctx, item, item.Version-1); err != nil {
			return err
		}
		if err := tx.RemediationIntents().Create(ctx, record); err != nil {
			return err
		}
		return tx.Approvals().Create(ctx, approval)
	}); err != nil {
		t.Fatal(err)
	}
	return item, approval
}

func approvalCode(err error, code common.ErrorCode) bool {
	var domainErr *common.DomainError
	return errors.As(err, &domainErr) && domainErr.Code == code
}
