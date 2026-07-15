package mock

import (
	"context"
	"fmt"
	"sync"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/orderdemo"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

type Scenario string

const (
	Healthy          Scenario = "healthy"
	WorkerStopped    Scenario = "worker-stopped"
	SlowProcessing   Scenario = "slow-processing"
	DependencyErrors Scenario = "dependency-errors"
)

type Adapter struct {
	mu            sync.Mutex
	scenario      Scenario
	now           time.Time
	workerVersion int
	operations    map[string]orderdemo.Operation
}

var _ orderdemo.Port = (*Adapter)(nil)

func New(scenario Scenario, now time.Time) (*Adapter, error) {
	switch scenario {
	case Healthy, WorkerStopped, SlowProcessing, DependencyErrors:
	default:
		return nil, fmt.Errorf("unknown order demo mock scenario %q", scenario)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	version := 1
	if scenario == WorkerStopped {
		version = 2
	}
	return &Adapter{scenario: scenario, now: now.UTC(), workerVersion: version, operations: make(map[string]orderdemo.Operation)}, nil
}

func (a *Adapter) GetRuntime(context.Context, requestcontext.Context) (orderdemo.Runtime, error) {
	return orderdemo.Runtime{ServiceRef: "order-demo", InstanceEpoch: "mock-epoch-1", StartedAt: a.now.Add(-time.Hour), SupervisorStatus: "running"}, nil
}

func (a *Adapter) GetQueue(context.Context, requestcontext.Context) (orderdemo.Queue, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	queue := orderdemo.Queue{Capacity: 100, ObservedAt: a.now}
	if a.scenario != Healthy {
		queue.Depth, queue.OldestAgeSeconds = 24, 42
	}
	return queue, nil
}

func (a *Adapter) GetWorker(context.Context, requestcontext.Context) (orderdemo.Worker, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	worker := orderdemo.Worker{ServiceRef: "order-demo", InstanceEpoch: "mock-epoch-1", ConfiguredConcurrency: 2, EffectiveConcurrency: 2, ActiveWorkers: 2, Version: a.workerVersion, ObservedAt: a.now}
	if a.scenario == WorkerStopped {
		worker.ConfiguredConcurrency, worker.EffectiveConcurrency, worker.ActiveWorkers, worker.Version = 0, 0, 0, 2
	}
	return worker, nil
}

func (a *Adapter) GetPolicy(context.Context, requestcontext.Context) (orderdemo.Policy, error) {
	return orderdemo.Policy{ServiceRef: "order-demo", ExpectedConcurrency: 2, MinConcurrency: 1, MaxConcurrency: 4, Version: "v1", Digest: "87f4ec09db9da8c9fdb1cb0c2d2857e21ce62b5c1d0c83a422b6d2c8536a9320"}, nil
}

func (a *Adapter) GetRecentOutcomes(_ context.Context, _ requestcontext.Context, request orderdemo.RecentRequest) ([]orderdemo.OrderOutcome, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if request.Limit < 1 || request.Limit > 20 || (request.Status != "" && !validStatus(request.Status)) {
		return nil, runtime.NewError(runtime.SchemaValidationFailed, "recent outcomes request is invalid", false)
	}
	status := "completed"
	var reason *string
	if a.scenario == DependencyErrors {
		status = "failed"
		value := "retry_exhausted"
		reason = &value
	} else if a.scenario != Healthy {
		status = "queued"
	}
	if request.Status != "" && request.Status != status {
		return []orderdemo.OrderOutcome{}, nil
	}
	return []orderdemo.OrderOutcome{{ID: "order-redacted-1", Status: status, CreatedAt: a.now.Add(-time.Minute), UpdatedAt: a.now, FailureReason: reason}}, nil
}

func (a *Adapter) GetOperation(_ context.Context, _ requestcontext.Context, operationID string) (orderdemo.Operation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	value, ok := a.operations[operationID]
	if !ok {
		return orderdemo.Operation{}, runtime.NewError(runtime.ResourceNotFound, "operation was not found", false)
	}
	return value, nil
}

func (a *Adapter) RestoreWorkerConcurrency(_ context.Context, _ requestcontext.Context, request orderdemo.RemediationRequest) (orderdemo.Operation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if existing, ok := a.operations[request.OperationID]; ok {
		if existing.InstanceEpoch != request.InstanceEpoch || existing.BeforeVersion != request.ExpectedVersion || existing.IntentDigest != request.IntentDigest || existing.ApprovalID != request.ApprovalID {
			return orderdemo.Operation{}, runtime.NewError(runtime.ResourceConflict, "operation ID was reused with different input", false)
		}
		return existing, nil
	}
	if a.scenario != WorkerStopped || request.InstanceEpoch != "mock-epoch-1" || request.ExpectedVersion != 2 || request.ExpectedConcurrency != 0 || request.NewConcurrency != 2 || request.ApprovalID == "" || len(request.IntentDigest) != 71 {
		return orderdemo.Operation{}, runtime.NewError(runtime.TargetPreconditionFailed, "mock remediation precondition failed", false)
	}
	value := orderdemo.Operation{OperationID: request.OperationID, InstanceEpoch: request.InstanceEpoch, BeforeVersion: 2, AfterVersion: 3, BeforeConcurrency: 0, AfterConcurrency: 2, IntentDigest: request.IntentDigest, ApprovalID: request.ApprovalID, ExecutedAt: a.now}
	a.operations[request.OperationID] = value
	a.scenario = Healthy
	a.workerVersion = 3
	return value, nil
}

func (a *Adapter) RunBusinessProbe(_ context.Context, _ requestcontext.Context, probeID string) (orderdemo.ProbeResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if probeID == "" {
		return orderdemo.ProbeResult{}, runtime.NewError(runtime.SchemaValidationFailed, "probe ID is invalid", false)
	}
	completedAt := a.now.Add(200 * time.Millisecond)
	result := "completed"
	duration := 200
	if a.scenario != Healthy {
		result, duration, completedAt = "timed_out", 5000, time.Time{}
	}
	var completed *time.Time
	if !completedAt.IsZero() {
		completed = &completedAt
	}
	return orderdemo.ProbeResult{ProbeID: probeID, Result: result, DurationMS: duration, CompletedAt: completed}, nil
}

func validStatus(status string) bool {
	switch status {
	case "queued", "processing", "completed", "failed":
		return true
	default:
		return false
	}
}
