package mock

import (
	"context"
	"fmt"
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
	scenario Scenario
	now      time.Time
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
	return &Adapter{scenario: scenario, now: now.UTC()}, nil
}

func (a *Adapter) GetRuntime(context.Context, requestcontext.Context) (orderdemo.Runtime, error) {
	return orderdemo.Runtime{ServiceRef: "order-demo", InstanceEpoch: "mock-epoch-1", StartedAt: a.now.Add(-time.Hour), SupervisorStatus: "running"}, nil
}

func (a *Adapter) GetQueue(context.Context, requestcontext.Context) (orderdemo.Queue, error) {
	queue := orderdemo.Queue{Capacity: 100, ObservedAt: a.now}
	if a.scenario != Healthy {
		queue.Depth, queue.OldestAgeSeconds = 24, 42
	}
	return queue, nil
}

func (a *Adapter) GetWorker(context.Context, requestcontext.Context) (orderdemo.Worker, error) {
	worker := orderdemo.Worker{ServiceRef: "order-demo", InstanceEpoch: "mock-epoch-1", ConfiguredConcurrency: 2, EffectiveConcurrency: 2, ActiveWorkers: 2, Version: 1, ObservedAt: a.now}
	if a.scenario == WorkerStopped {
		worker.ConfiguredConcurrency, worker.EffectiveConcurrency, worker.ActiveWorkers, worker.Version = 0, 0, 0, 2
	}
	return worker, nil
}

func (a *Adapter) GetPolicy(context.Context, requestcontext.Context) (orderdemo.Policy, error) {
	return orderdemo.Policy{ServiceRef: "order-demo", ExpectedConcurrency: 2, MinConcurrency: 1, MaxConcurrency: 4, Version: "v1", Digest: "87f4ec09db9da8c9fdb1cb0c2d2857e21ce62b5c1d0c83a422b6d2c8536a9320"}, nil
}

func (a *Adapter) GetRecentOutcomes(_ context.Context, _ requestcontext.Context, request orderdemo.RecentRequest) ([]orderdemo.OrderOutcome, error) {
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

func (a *Adapter) GetOperation(context.Context, requestcontext.Context, string) (orderdemo.Operation, error) {
	return orderdemo.Operation{}, runtime.NewError(runtime.ResourceNotFound, "operation was not found", false)
}

func validStatus(status string) bool {
	switch status {
	case "queued", "processing", "completed", "failed":
		return true
	default:
		return false
	}
}
