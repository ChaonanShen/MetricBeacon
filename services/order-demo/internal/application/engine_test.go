package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"mini-torchbearing.local/services/order-demo/internal/domain/order"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	var id atomic.Int64
	e := NewEngine(Options{
		ProcessingDelay: 5 * time.Millisecond,
		SlowDelay:       80 * time.Millisecond,
		DependencyDelay: 20 * time.Millisecond,
		ProbeTimeout:    150 * time.Millisecond,
		ID:              func() string { return fmt.Sprintf("id-%d", id.Add(1)) },
	})
	t.Cleanup(e.Close)
	return e
}

func TestHealthyWorkersCompleteRealOrders(t *testing.T) {
	e := testEngine(t)
	created, err := e.Submit("key-1", "DEMO", 1)
	if err != nil {
		t.Fatal(err)
	}
	got := waitStatus(t, e, created.ID, order.StatusCompleted)
	if got.FailureReason != "" {
		t.Fatalf("failure = %q", got.FailureReason)
	}
	if workers := e.WorkerSnapshot(); workers.ConfiguredConcurrency != 2 || workers.ActiveWorkers != 2 {
		t.Fatalf("workers = %+v", workers)
	}
}

func TestStoppedWorkersCauseAndThenDrainRealBacklog(t *testing.T) {
	e := testEngine(t)
	if err := e.ActivateFault(FaultWorkerStopped); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return e.WorkerSnapshot().ActiveWorkers == 0 })
	created := make([]OrderSnapshot, 3)
	for i := range created {
		var err error
		created[i], err = e.Submit(fmt.Sprintf("key-%d", i), "DEMO", 1)
		if err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(25 * time.Millisecond)
	if queue := e.QueueSnapshot(); queue.Depth != 3 || queue.OldestAgeSeconds <= 0 {
		t.Fatalf("queue = %+v", queue)
	}
	stopped := e.WorkerSnapshot()
	receipt, err := e.Remediate(RemediationRequest{OperationID: "op-1", InstanceEpoch: stopped.InstanceEpoch, ExpectedVersion: stopped.Version,
		ExpectedConcurrency: 0, NewConcurrency: 2, IntentDigest: strings.Repeat("a", 64), ApprovalID: "approval-1"})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.BeforeConcurrency != 0 || receipt.AfterConcurrency != 2 {
		t.Fatalf("receipt = %+v", receipt)
	}
	for _, item := range created {
		waitStatus(t, e, item.ID, order.StatusCompleted)
	}
	waitFor(t, func() bool { return e.QueueSnapshot().Depth == 0 })
}

func TestRemediationIsCASBoundAndIdempotent(t *testing.T) {
	e := testEngine(t)
	_ = e.ActivateFault(FaultWorkerStopped)
	stopped := e.WorkerSnapshot()
	request := RemediationRequest{OperationID: "op-1", InstanceEpoch: stopped.InstanceEpoch, ExpectedVersion: stopped.Version,
		ExpectedConcurrency: 0, NewConcurrency: 2, IntentDigest: strings.Repeat("b", 64), ApprovalID: "approval-1"}
	first, err := e.Remediate(request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.Remediate(request)
	if err != nil || first != second {
		t.Fatalf("idempotent result = %+v, %v", second, err)
	}
	request.ApprovalID = "approval-2"
	if _, err := e.Remediate(request); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed operation error = %v", err)
	}
	request.OperationID = "op-2"
	if _, err := e.Remediate(request); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("stale version error = %v", err)
	}
}

func TestSimilarFaultsKeepWorkersActiveAndDoNotChangeConfiguration(t *testing.T) {
	for _, scenario := range []string{FaultSlowProcessing, FaultDependencyErrors} {
		t.Run(scenario, func(t *testing.T) {
			e := testEngine(t)
			before := e.WorkerSnapshot()
			if err := e.ActivateFault(scenario); err != nil {
				t.Fatal(err)
			}
			after := e.WorkerSnapshot()
			if after.ConfiguredConcurrency != 2 || after.Version != before.Version || after.ActiveWorkers != 2 {
				t.Fatalf("workers = %+v", after)
			}
		})
	}
}

func TestBusinessProbeUsesTheRealQueue(t *testing.T) {
	e := testEngine(t)
	result := e.RunProbe(context.Background(), "probe-1")
	if result.Result != "completed" || result.OrderID == "" || result.CompletedAt == nil {
		t.Fatalf("probe = %+v", result)
	}
	_ = e.ActivateFault(FaultWorkerStopped)
	waitFor(t, func() bool { return e.WorkerSnapshot().ActiveWorkers == 0 })
	timedOut := e.RunProbe(context.Background(), "probe-2")
	if timedOut.Result != "timed_out" {
		t.Fatalf("probe = %+v", timedOut)
	}
}

func waitStatus(t *testing.T, e *Engine, id string, wanted order.Status) OrderSnapshot {
	t.Helper()
	var result OrderSnapshot
	waitFor(t, func() bool {
		result, _ = e.GetOrder(id)
		return result.Status == wanted
	})
	return result
}

func waitFor(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
