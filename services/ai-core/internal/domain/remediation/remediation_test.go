package remediation

import (
	"testing"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

func TestApprovalDecisionAndExpiryAreSingleTransition(t *testing.T) {
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name     string
		decision Decision
		at       time.Time
		want     ApprovalStatus
	}{
		{"approve", DecisionApprove, now.Add(time.Minute), ApprovalApproved},
		{"reject", DecisionReject, now.Add(time.Minute), ApprovalRejected},
		{"expired approval cannot become approved", DecisionApprove, now.Add(DefaultApprovalTTL), ApprovalExpired},
	} {
		t.Run(test.name, func(t *testing.T) {
			value, err := NewApproval("approval-1", "org:1", "1", "task-1", "intent-1", digest('a'), now)
			if err != nil {
				t.Fatal(err)
			}
			if err := value.Decide(test.decision, "admin:1", "reviewed", test.at); err != nil {
				t.Fatal(err)
			}
			if value.Status != test.want || value.Version != 2 || !value.Valid() {
				t.Fatalf("approval=%#v", value)
			}
			if err := value.Decide(test.decision, "admin:1", "again", test.at.Add(time.Second)); err == nil {
				t.Fatal("second decision was accepted")
			}
		})
	}
}

func TestExecutionRequiresExactReceiptAndReconcilesUnknown(t *testing.T) {
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	value, err := NewExecution("operation-1", "org:1", "1", "task-1", "approval-1", digest('b'), "epoch-1", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := value.RecordReceipt(3, 4, false, now.Add(time.Second)); err == nil {
		t.Fatal("mismatched receipt was accepted")
	}
	if err := value.MarkUnknown(now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := value.RecordReceipt(2, 3, true, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if value.State != ExecutionAlreadyApplied || value.Version != 3 || !value.Valid() {
		t.Fatalf("execution=%#v", value)
	}
	if err := value.MarkFailed(common.DependencyUnavailable, now.Add(3*time.Second)); err == nil {
		t.Fatal("terminal execution was changed")
	}
}

func TestIntentAndAuditAreClosedWorld(t *testing.T) {
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	intent := Intent{ID: "intent-1", Digest: digest('c'), CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: "order-demo", InstanceEpoch: "epoch-1", ExpectedVersion: 2, BeforeConcurrency: 0, AfterConcurrency: 2, Risk: "low", CreatedAt: now}
	if !intent.Valid("order-demo") {
		t.Fatal("valid fixed intent was rejected")
	}
	intent.AfterConcurrency = 3
	if intent.Valid("order-demo") {
		t.Fatal("unregistered write shape was accepted")
	}
	if _, err := NewAuditRecord("audit-1", "org:1", "1", "task-1", "admin:1", "arbitrary", AuditSucceeded, "done", now); err == nil {
		t.Fatal("arbitrary audit action was accepted")
	}
}

func digest(char byte) string {
	value := make([]byte, 64)
	for index := range value {
		value[index] = char
	}
	return "sha256:" + string(value)
}
