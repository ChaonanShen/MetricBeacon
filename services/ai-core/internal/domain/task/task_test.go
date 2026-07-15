package task

import (
	"testing"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

func TestTaskStateMachine(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC)
	range30m, err := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	if err != nil {
		t.Fatal(err)
	}
	task, err := New("task_1", "org:1", "session_1", "message_1", "prometheus-main", range30m, LegacyQueryPlan(), now)
	if err != nil {
		t.Fatal(err)
	}
	if task.Kind != KindMetricAnalysis || task.IncidentPlan != nil {
		t.Fatalf("metric task discriminator = %#v", task)
	}
	for _, next := range []Status{StatusPlanning, StatusRunningTools, StatusValidating, StatusCompleted} {
		if err := task.Transition(next, now); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
	if task.StartedAt == nil || task.CompletedAt == nil {
		t.Fatal("terminal task must have timestamps")
	}
	if err := task.Transition(StatusFailed, now); err == nil {
		t.Fatal("terminal task transition unexpectedly succeeded")
	}
}

func TestIncidentStateMachineRequiresIntentBeforeApproval(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	value, err := NewIncident("task_1", "org:1", "session_1", "message_1", incidentPlan(), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, next := range []Status{StatusPlanning, StatusRunningTools} {
		if err := value.Transition(next, now); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
	if err := value.Transition(StatusWaitingApproval, now); err == nil {
		t.Fatal("approval wait without an Intent unexpectedly succeeded")
	}
	diagnosis := Diagnosis{PrimaryHypothesis: "worker_stopped", EvidenceRefs: []string{"worker-config", "queue-runtime"}, AlternativeHypotheses: []string{"slow_processing"}, Confidence: 0.99, CandidateAction: "restore_worker_concurrency"}
	if err := value.RecordDiagnosis(diagnosis, now); err != nil {
		t.Fatal(err)
	}
	intent := RemediationIntent{ID: "intent_1", Digest: digest('e'), CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: "order-service:demo", InstanceEpoch: "epoch_1", ExpectedVersion: 2, BeforeConcurrency: 0, AfterConcurrency: 2, Risk: "low", CreatedAt: now}
	if err := value.RecordIntent(intent, now); err != nil {
		t.Fatal(err)
	}
	if err := value.Transition(StatusWaitingApproval, now); err != nil {
		t.Fatal(err)
	}
	for _, next := range []Status{StatusExecuting, StatusReconciling, StatusValidating, StatusCompleted} {
		if err := value.Transition(next, now); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
	if value.CompletedAt == nil || value.IncidentPlan.Intent == nil || value.IncidentPlan.Phase != PhaseCompleted {
		t.Fatalf("completed Incident = %#v", value)
	}
}

func TestIncidentNoActionSeparatesSimilarSymptoms(t *testing.T) {
	for _, hypothesis := range []string{"slow_processing", "dependency_errors", "healthy", "insufficient_evidence"} {
		t.Run(hypothesis, func(t *testing.T) {
			now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
			value, err := NewIncident("task_1", "org:1", "session_1", "message_1", incidentPlan(), now)
			if err != nil {
				t.Fatal(err)
			}
			_ = value.Transition(StatusPlanning, now)
			_ = value.Transition(StatusRunningTools, now)
			if err := value.RecordDiagnosis(Diagnosis{PrimaryHypothesis: hypothesis, EvidenceRefs: []string{"runtime"}, Confidence: 0.8, CandidateAction: "no_action"}, now); err != nil {
				t.Fatal(err)
			}
			if err := value.CompleteNoAction(now); err != nil {
				t.Fatal(err)
			}
			if value.Status != StatusCompleted || value.IncidentPlan.Phase != PhaseNoAction || value.IncidentPlan.Intent != nil {
				t.Fatalf("no-action Incident = %#v", value)
			}
		})
	}
}

func TestIncidentPlanRejectsMetricAndUnpinnedInputs(t *testing.T) {
	now := time.Now()
	plan := incidentPlan()
	plan.AssetRefs[0].Digest = "not-a-digest"
	if _, err := NewIncident("task_1", "org:1", "session_1", "message_1", plan, now); err == nil {
		t.Fatal("unpinned asset unexpectedly succeeded")
	}
	metric, _ := common.NewAbsoluteTimeRange(now.Add(-time.Minute), now)
	value, _ := New("task_2", "org:1", "session_2", "message_2", "prometheus-main", metric, LegacyQueryPlan(), now)
	if err := value.Transition(StatusWaitingApproval, now); err == nil {
		t.Fatal("metric task entered approval state")
	}
}

func incidentPlan() IncidentPlan {
	return IncidentPlan{
		SourceID: "demo-grafana", AlertName: "OrderQueueBacklog", AlertFingerprint: "fingerprint_1", ServiceRef: "order-service:demo",
		Labels:  map[string]string{"alertname": "OrderQueueBacklog", "service_ref": "order-service:demo"},
		Mapping: PinnedRef{ID: "order-demo", Digest: digest('a')}, Playbook: PinnedRef{ID: "order-queue-backlog", Version: "1", Digest: digest('b')},
		AssetRefs: []AssetRef{{Kind: "alert_mapping", ID: "order-demo", Version: "1", Digest: digest('a')}, {Kind: "playbook", ID: "order-queue-backlog", Version: "1", Digest: digest('b')}, {Kind: "knowledge", ID: "order-service", Version: "1", Digest: digest('c')}, {Kind: "skill", ID: "diagnose-order-backlog", Version: "1", Digest: digest('d')}},
		Phase:     PhaseNeedsAgent,
	}
}

func digest(char byte) string {
	return "sha256:" + string(makeBytes(char, 64))
}

func makeBytes(char byte, count int) []byte {
	value := make([]byte, count)
	for index := range value {
		value[index] = char
	}
	return value
}

func TestTaskFailureFromNonTerminal(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC)
	range30m, _ := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	task, _ := New("task_1", "org:1", "session_1", "message_1", "prometheus-main", range30m, LegacyQueryPlan(), now)
	err := task.Fail(common.NewError(common.DependencyUnavailable, "assistant-mcp is unavailable", true), now)
	if err != nil || task.Status != StatusFailed || task.Error == nil {
		t.Fatalf("unexpected failed task: %#v, %v", task, err)
	}
}
