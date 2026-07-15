package playbook_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/adapters/assets/filesystem"
	ordermock "mini-torchbearing.local/services/assistant-mcp/internal/adapters/orderdemo/mock"
	"mini-torchbearing.local/services/assistant-mcp/internal/playbook"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/assets"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/orderdemo"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

var checkpointKey = []byte("0123456789abcdef0123456789abcdef")

func TestOnlyStoppedWorkerProducesVersionBoundIntent(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		scenario   ordermock.Scenario
		wantIntent bool
	}{
		{ordermock.WorkerStopped, true},
		{ordermock.Healthy, false},
		{ordermock.SlowProcessing, false},
		{ordermock.DependencyErrors, false},
	} {
		t.Run(string(test.scenario), func(t *testing.T) {
			engine := newEngine(t, test.scenario, now, func() time.Time { return now })
			start := startRun(t, engine)
			if start.Status != "needs_agent" || len(start.AssetRefs) != 3 || len(start.CapabilityIDs) != 6 || len(start.Checkpoint) > 4096 {
				t.Fatalf("unexpected start result: %#v", start)
			}
			result, err := engine.Resume(context.Background(), identity(), start.Checkpoint, restoreDiagnosis())
			if err != nil {
				t.Fatal(err)
			}
			if (result.IntentDraft != nil) != test.wantIntent {
				t.Fatalf("unexpected prepare result: %#v", result)
			}
			if test.wantIntent {
				intent := result.IntentDraft
				if result.Status != "needs_approval" || intent.CapabilityID != "order_service.restore_worker_concurrency" || intent.InstanceEpoch != "mock-epoch-1" || intent.ExpectedVersion != 2 || intent.BeforeConcurrency != 0 || intent.AfterConcurrency != 2 || len(intent.PolicyDigest) != 64 || len(intent.PlaybookDigest) != 64 || !intent.ObservedAt.Equal(now) {
					t.Fatalf("intent is not version-bound: %#v", intent)
				}
			} else if result.Status != "completed" {
				t.Fatalf("non-action did not complete: %#v", result)
			}
		})
	}
}

func TestDiagnosisCannotOverrideDeterministicPreparePolicy(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		diagnosis playbook.Diagnosis
	}{
		{"explicit no action", playbook.Diagnosis{PrimaryHypothesis: "already recovering", EvidenceRefs: []string{"order_service.get_worker_state"}, Confidence: 1, CandidateAction: "no_action"}},
		{"low confidence", func() playbook.Diagnosis { value := restoreDiagnosis(); value.Confidence = 0.79; return value }()},
		{"missing policy evidence", func() playbook.Diagnosis {
			value := restoreDiagnosis()
			value.EvidenceRefs = []string{"order_service.get_worker_state"}
			return value
		}()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := newEngine(t, ordermock.WorkerStopped, now, func() time.Time { return now })
			result, err := engine.Resume(context.Background(), identity(), startRun(t, engine).Checkpoint, test.diagnosis)
			if err != nil || result.Status != "completed" || result.IntentDraft != nil {
				t.Fatalf("unsafe intent created: %#v %v", result, err)
			}
		})
	}
}

func TestCheckpointIntegrityFreshnessAndPhaseAreEnforced(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	current := now
	engine := newEngine(t, ordermock.WorkerStopped, now, func() time.Time { return current })
	start := startRun(t, engine)

	tampered := start.Checkpoint[:len(start.Checkpoint)-1] + "A"
	_, err := engine.Resume(context.Background(), identity(), tampered, restoreDiagnosis())
	requireCode(t, err, runtime.SchemaValidationFailed)

	otherKeyEngine, err := playbook.NewEngine(assetStore(t), mockPort(t, ordermock.WorkerStopped, now), []byte("abcdef0123456789abcdef0123456789"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	_, err = otherKeyEngine.Resume(context.Background(), identity(), start.Checkpoint, restoreDiagnosis())
	requireCode(t, err, runtime.SchemaValidationFailed)

	current = now.Add(61 * time.Second)
	_, err = engine.Resume(context.Background(), identity(), start.Checkpoint, restoreDiagnosis())
	requireCode(t, err, runtime.InvalidStateTransition)

	current = now
	prepared, err := engine.Resume(context.Background(), identity(), start.Checkpoint, restoreDiagnosis())
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Resume(context.Background(), identity(), prepared.Checkpoint, restoreDiagnosis())
	requireCode(t, err, runtime.InvalidStateTransition)
}

func TestStrictDiagnosisRejectsUnknownEvidenceAndMalformedValues(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	engine := newEngine(t, ordermock.WorkerStopped, now, func() time.Time { return now })
	checkpoint := startRun(t, engine).Checkpoint
	for _, diagnosis := range []playbook.Diagnosis{
		{PrimaryHypothesis: "stopped", EvidenceRefs: []string{"fault.ground_truth"}, Confidence: 1, CandidateAction: "restore_worker_concurrency"},
		{PrimaryHypothesis: "", EvidenceRefs: []string{"order_service.get_worker_state"}, Confidence: 1, CandidateAction: "restore_worker_concurrency"},
		{PrimaryHypothesis: "stopped", EvidenceRefs: []string{"order_service.get_worker_state"}, Confidence: 2, CandidateAction: "restore_worker_concurrency"},
		{PrimaryHypothesis: "stopped", EvidenceRefs: []string{}, Confidence: 1, CandidateAction: "restore_worker_concurrency"},
	} {
		_, err := engine.Resume(context.Background(), identity(), checkpoint, diagnosis)
		requireCode(t, err, runtime.SchemaValidationFailed)
	}
}

func TestStartRejectsEpochMismatchAndInvalidConstruction(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	if _, err := playbook.NewEngine(nil, nil, nil, nil); err == nil {
		t.Fatal("invalid engine construction was accepted")
	}
	base := mockPort(t, ordermock.WorkerStopped, now)
	engine, err := playbook.NewEngine(assetStore(t), epochMismatchPort{Port: base}, checkpointKey, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	resolution, _ := assetStore(t).Resolve(assets.Alert{SourceID: "demo-grafana", AlertName: "OrderQueueBacklog", Labels: map[string]string{"service_ref": "order-demo"}})
	_, err = engine.Start(context.Background(), identity(), resolution.PlaybookID, resolution.PlaybookVersion, resolution.PlaybookDigest, resolution.ServiceRef)
	requireCode(t, err, runtime.SchemaValidationFailed)
}

type epochMismatchPort struct{ orderdemo.Port }

func (p epochMismatchPort) GetRuntime(ctx context.Context, identity requestcontext.Context) (orderdemo.Runtime, error) {
	value, err := p.Port.GetRuntime(ctx, identity)
	value.InstanceEpoch = "different-epoch"
	return value, err
}

func newEngine(t *testing.T, scenario ordermock.Scenario, observed time.Time, now func() time.Time) *playbook.Engine {
	t.Helper()
	engine, err := playbook.NewEngine(assetStore(t), mockPort(t, scenario, observed), checkpointKey, now)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func mockPort(t *testing.T, scenario ordermock.Scenario, observed time.Time) orderdemo.Port {
	t.Helper()
	port, err := ordermock.New(scenario, observed)
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func assetStore(t *testing.T) assets.Port {
	t.Helper()
	root := repositoryRoot(t)
	store, err := filesystem.New(filepath.Join(root, "data", "operational-assets"), filepath.Join(root, "contracts", "schemas", "assets"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func startRun(t *testing.T, engine *playbook.Engine) playbook.StartResult {
	t.Helper()
	store := assetStore(t)
	resolution, err := store.Resolve(assets.Alert{SourceID: "demo-grafana", AlertName: "OrderQueueBacklog", Labels: map[string]string{"service_ref": "order-demo"}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Start(context.Background(), identity(), resolution.PlaybookID, resolution.PlaybookVersion, resolution.PlaybookDigest, resolution.ServiceRef)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func restoreDiagnosis() playbook.Diagnosis {
	return playbook.Diagnosis{PrimaryHypothesis: "worker concurrency is stopped", EvidenceRefs: []string{"order_service.get_worker_state", "order_service.get_worker_policy", "order_service.get_queue_snapshot"}, Alternatives: []string{"slow processing"}, Confidence: 0.95, CandidateAction: "restore_worker_concurrency"}
}

func identity() requestcontext.Context {
	return requestcontext.Context{TenantID: "org:1", OrgID: "1", UserID: "user:1"}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.work")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("repository root not found")
		}
		directory = parent
	}
}

func requireCode(t *testing.T, err error, want runtime.ErrorCode) {
	t.Helper()
	var toolError *runtime.ToolError
	if !errors.As(err, &toolError) || toolError.Code != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
