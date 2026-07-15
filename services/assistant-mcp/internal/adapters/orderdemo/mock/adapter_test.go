package mock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/adapters/orderdemo/mock"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/orderdemo"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

func TestScenariosPreserveDistinctDiagnosticEvidence(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		scenario          mock.Scenario
		wantConfigured    int
		wantActive        int
		wantOutcomeStatus string
		wantFailure       bool
	}{
		{mock.Healthy, 2, 2, "completed", false},
		{mock.WorkerStopped, 0, 0, "queued", false},
		{mock.SlowProcessing, 2, 2, "queued", false},
		{mock.DependencyErrors, 2, 2, "failed", true},
	}
	for _, test := range tests {
		t.Run(string(test.scenario), func(t *testing.T) {
			adapter, err := mock.New(test.scenario, now)
			if err != nil {
				t.Fatal(err)
			}
			assertReadContract(t, adapter, now)
			worker, _ := adapter.GetWorker(context.Background(), requestcontext.Context{})
			outcomes, err := adapter.GetRecentOutcomes(context.Background(), requestcontext.Context{}, orderdemo.RecentRequest{Limit: 10})
			if err != nil || worker.ConfiguredConcurrency != test.wantConfigured || worker.ActiveWorkers != test.wantActive || len(outcomes) != 1 || outcomes[0].Status != test.wantOutcomeStatus || (outcomes[0].FailureReason != nil) != test.wantFailure {
				t.Fatalf("unexpected evidence: worker=%#v outcomes=%#v err=%v", worker, outcomes, err)
			}
		})
	}
}

func TestMockRejectsInvalidScenarioAndBoundedInputs(t *testing.T) {
	if _, err := mock.New("ground-truth", time.Now()); err == nil {
		t.Fatal("unknown scenario was accepted")
	}
	adapter, _ := mock.New(mock.Healthy, time.Now())
	for _, request := range []orderdemo.RecentRequest{{Limit: 0}, {Limit: 21}, {Limit: 1, Status: "unknown"}} {
		_, err := adapter.GetRecentOutcomes(context.Background(), requestcontext.Context{}, request)
		requireCode(t, err, runtime.SchemaValidationFailed)
	}
	_, err := adapter.GetOperation(context.Background(), requestcontext.Context{}, "missing")
	requireCode(t, err, runtime.ResourceNotFound)
}

func assertReadContract(t *testing.T, adapter orderdemo.Port, now time.Time) {
	t.Helper()
	ctx := context.Background()
	identity := requestcontext.Context{TenantID: "org:1"}
	runtimeState, err := adapter.GetRuntime(ctx, identity)
	if err != nil || runtimeState.ServiceRef != "order-demo" || runtimeState.InstanceEpoch == "" {
		t.Fatalf("runtime contract failed: %#v %v", runtimeState, err)
	}
	queue, err := adapter.GetQueue(ctx, identity)
	if err != nil || queue.Capacity != 100 || !queue.ObservedAt.Equal(now) {
		t.Fatalf("queue contract failed: %#v %v", queue, err)
	}
	policy, err := adapter.GetPolicy(ctx, identity)
	if err != nil || policy.ExpectedConcurrency != 2 || policy.Version != "v1" || len(policy.Digest) != 64 {
		t.Fatalf("policy contract failed: %#v %v", policy, err)
	}
}

func requireCode(t *testing.T, err error, want runtime.ErrorCode) {
	t.Helper()
	var toolError *runtime.ToolError
	if !errors.As(err, &toolError) || toolError.Code != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
