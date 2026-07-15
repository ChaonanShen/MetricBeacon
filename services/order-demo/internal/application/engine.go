package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"mini-torchbearing.local/services/order-demo/internal/domain/order"
	"mini-torchbearing.local/services/order-demo/internal/domain/worker"
	metricport "mini-torchbearing.local/services/order-demo/internal/ports/metrics"
)

const (
	ServiceRef            = "order-demo"
	QueueCapacity         = 100
	FaultWorkerStopped    = "worker-stopped"
	FaultSlowProcessing   = "slow-processing"
	FaultDependencyErrors = "dependency-errors"
	FailureDependency     = "dependency_unavailable"
	FailureRetryExhausted = "retry_exhausted"
)

var (
	ErrInvalidArgument     = errors.New("invalid argument")
	ErrQueueFull           = errors.New("order queue is full")
	ErrNotFound            = errors.New("resource not found")
	ErrIdempotencyConflict = errors.New("idempotency conflict")
	ErrPreconditionFailed  = errors.New("target precondition failed")
	ErrInvalidFault        = errors.New("invalid fault scenario")
)

type Options struct {
	ProcessingDelay time.Duration
	SlowDelay       time.Duration
	DependencyDelay time.Duration
	ProbeTimeout    time.Duration
	Metrics         metricport.Recorder
	Now             func() time.Time
	ID              func() string
}

type OrderSnapshot struct {
	ID            string
	Status        order.Status
	FailureReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type QueueSnapshot struct {
	Depth            int
	Capacity         int
	OldestAgeSeconds float64
	ObservedAt       time.Time
}

type WorkerSnapshot struct {
	ServiceRef            string
	InstanceEpoch         string
	ConfiguredConcurrency int
	EffectiveConcurrency  int
	ActiveWorkers         int
	InflightOrders        int
	Version               int
	ObservedAt            time.Time
}

type RemediationRequest struct {
	OperationID         string
	InstanceEpoch       string
	ExpectedVersion     int
	ExpectedConcurrency int
	NewConcurrency      int
	IntentDigest        string
	ApprovalID          string
}

type OperationReceipt struct {
	OperationID       string
	InstanceEpoch     string
	BeforeVersion     int
	AfterVersion      int
	BeforeConcurrency int
	AfterConcurrency  int
	IntentDigest      string
	ApprovalID        string
	ExecutedAt        time.Time
}

type ProbeResult struct {
	ProbeID     string
	OrderID     string
	Result      string
	Duration    time.Duration
	CompletedAt *time.Time
}

type idempotentOrder struct {
	payloadDigest string
	orderID       string
}

type operationEntry struct {
	payloadDigest string
	receipt       OperationReceipt
}

type Engine struct {
	mu sync.RWMutex

	queue      chan string
	orders     map[string]*order.Order
	done       map[string]chan struct{}
	idempotent map[string]idempotentOrder
	operations map[string]operationEntry
	workers    map[int]context.CancelFunc
	faults     map[string]bool
	nextWorker int
	active     int
	inflight   int
	config     worker.Config
	startedAt  time.Time

	processingDelay time.Duration
	slowDelay       time.Duration
	dependencyDelay time.Duration
	probeTimeout    time.Duration
	metrics         metricport.Recorder
	now             func() time.Time
	id              func() string
}

func NewEngine(options Options) *Engine {
	if options.ProcessingDelay <= 0 {
		options.ProcessingDelay = 200 * time.Millisecond
	}
	if options.SlowDelay <= 0 {
		options.SlowDelay = 2 * time.Second
	}
	if options.DependencyDelay <= 0 {
		options.DependencyDelay = time.Second
	}
	if options.ProbeTimeout <= 0 {
		options.ProbeTimeout = 5 * time.Second
	}
	if options.Metrics == nil {
		options.Metrics = metricport.Noop{}
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.ID == nil {
		options.ID = randomID
	}
	started := options.Now().UTC()
	e := &Engine{
		queue: make(chan string, QueueCapacity), orders: make(map[string]*order.Order), done: make(map[string]chan struct{}),
		idempotent: make(map[string]idempotentOrder), operations: make(map[string]operationEntry), workers: make(map[int]context.CancelFunc), faults: make(map[string]bool),
		config: worker.NewConfig(options.ID()), startedAt: started, processingDelay: options.ProcessingDelay, slowDelay: options.SlowDelay,
		dependencyDelay: options.DependencyDelay, probeTimeout: options.ProbeTimeout, metrics: options.Metrics, now: options.Now, id: options.ID,
	}
	e.mu.Lock()
	e.resizeLocked(worker.ExpectedConcurrency)
	e.mu.Unlock()
	return e
}

func (e *Engine) Close() {
	e.mu.Lock()
	for id, cancel := range e.workers {
		cancel()
		delete(e.workers, id)
	}
	e.mu.Unlock()
}

func (e *Engine) StartedAt() time.Time { return e.startedAt }

func (e *Engine) Submit(idempotencyKey, sku string, quantity int) (OrderSnapshot, error) {
	if idempotencyKey == "" || sku == "" || quantity < 1 || quantity > 10 {
		return OrderSnapshot{}, ErrInvalidArgument
	}
	payload := digest(struct {
		SKU      string
		Quantity int
	}{sku, quantity})
	e.mu.Lock()
	if previous, ok := e.idempotent[idempotencyKey]; ok {
		if previous.payloadDigest != payload {
			e.mu.Unlock()
			return OrderSnapshot{}, ErrIdempotencyConflict
		}
		snapshot := snapshotOrder(*e.orders[previous.orderID])
		e.mu.Unlock()
		return snapshot, nil
	}
	now := e.now().UTC()
	created := order.New(e.id(), sku, quantity, now)
	select {
	case e.queue <- created.ID:
		e.orders[created.ID] = &created
		e.done[created.ID] = make(chan struct{})
		e.idempotent[idempotencyKey] = idempotentOrder{payloadDigest: payload, orderID: created.ID}
		result := snapshotOrder(created)
		e.mu.Unlock()
		e.metrics.OrderReceived("accepted")
		return result, nil
	default:
		e.mu.Unlock()
		e.metrics.OrderReceived("rejected")
		return OrderSnapshot{}, ErrQueueFull
	}
}

func (e *Engine) GetOrder(id string) (OrderSnapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	o, ok := e.orders[id]
	if !ok {
		return OrderSnapshot{}, ErrNotFound
	}
	return snapshotOrder(*o), nil
}

func (e *Engine) Recent(status order.Status, limit int) []OrderSnapshot {
	if limit < 1 || limit > 20 {
		limit = 10
	}
	e.mu.RLock()
	items := make([]OrderSnapshot, 0, len(e.orders))
	for _, o := range e.orders {
		if status == "" || o.Status == status {
			items = append(items, snapshotOrder(*o))
		}
	}
	e.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (e *Engine) QueueSnapshot() QueueSnapshot {
	now := e.now().UTC()
	e.mu.RLock()
	oldest := time.Time{}
	for _, o := range e.orders {
		if o.Status == order.StatusQueued && (oldest.IsZero() || o.CreatedAt.Before(oldest)) {
			oldest = o.CreatedAt
		}
	}
	depth := len(e.queue)
	e.mu.RUnlock()
	age := 0.0
	if !oldest.IsZero() {
		age = max(0, now.Sub(oldest).Seconds())
	}
	return QueueSnapshot{Depth: depth, Capacity: QueueCapacity, OldestAgeSeconds: age, ObservedAt: now}
}

func (e *Engine) WorkerSnapshot() WorkerSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return WorkerSnapshot{ServiceRef: ServiceRef, InstanceEpoch: e.config.InstanceEpoch, ConfiguredConcurrency: e.config.Concurrency,
		EffectiveConcurrency: e.config.Concurrency, ActiveWorkers: e.active, InflightOrders: e.inflight, Version: e.config.Version, ObservedAt: e.now().UTC()}
}

func (e *Engine) Remediate(request RemediationRequest) (OperationReceipt, error) {
	if request.OperationID == "" || request.ApprovalID == "" || len(request.IntentDigest) != 64 {
		return OperationReceipt{}, ErrInvalidArgument
	}
	payload := digest(request)
	e.mu.Lock()
	if previous, ok := e.operations[request.OperationID]; ok {
		e.mu.Unlock()
		if previous.payloadDigest != payload {
			return OperationReceipt{}, ErrIdempotencyConflict
		}
		return previous.receipt, nil
	}
	beforeVersion, beforeConcurrency := e.config.Version, e.config.Concurrency
	if err := e.config.Restore(request.InstanceEpoch, request.ExpectedVersion, request.ExpectedConcurrency, request.NewConcurrency); err != nil {
		e.mu.Unlock()
		return OperationReceipt{}, ErrPreconditionFailed
	}
	e.resizeLocked(e.config.Concurrency)
	receipt := OperationReceipt{OperationID: request.OperationID, InstanceEpoch: request.InstanceEpoch, BeforeVersion: beforeVersion,
		AfterVersion: e.config.Version, BeforeConcurrency: beforeConcurrency, AfterConcurrency: e.config.Concurrency,
		IntentDigest: request.IntentDigest, ApprovalID: request.ApprovalID, ExecutedAt: e.now().UTC()}
	e.operations[request.OperationID] = operationEntry{payloadDigest: payload, receipt: receipt}
	e.mu.Unlock()
	return receipt, nil
}

func (e *Engine) GetOperation(id string) (OperationReceipt, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	entry, ok := e.operations[id]
	if !ok {
		return OperationReceipt{}, ErrNotFound
	}
	return entry.receipt, nil
}

func (e *Engine) ActivateFault(scenario string) error {
	if !validFault(scenario) {
		return ErrInvalidFault
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.faults[scenario] = true
	if scenario == FaultWorkerStopped {
		if err := e.config.SetInternal(0); err != nil {
			return err
		}
		e.resizeLocked(0)
	}
	return nil
}

func (e *Engine) ClearFault(scenario string) error {
	if !validFault(scenario) {
		return ErrInvalidFault
	}
	e.mu.Lock()
	delete(e.faults, scenario)
	e.mu.Unlock()
	return nil
}

func (e *Engine) ResetFaults() {
	e.mu.Lock()
	e.faults = make(map[string]bool)
	_ = e.config.SetInternal(worker.ExpectedConcurrency)
	e.resizeLocked(worker.ExpectedConcurrency)
	e.mu.Unlock()
}

func (e *Engine) FaultActive(scenario string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.faults[scenario]
}

func (e *Engine) RunProbe(ctx context.Context, probeID string) ProbeResult {
	started := e.now().UTC()
	result := ProbeResult{ProbeID: probeID}
	if probeID == "" {
		result.Result = "failed"
		e.metrics.BusinessProbe(result.Result)
		return result
	}
	snapshot, err := e.Submit("probe:"+probeID, "PROBE", 1)
	if err != nil {
		result.Result = "failed"
		e.metrics.BusinessProbe(result.Result)
		return result
	}
	result.OrderID = snapshot.ID
	e.mu.RLock()
	done := e.done[snapshot.ID]
	e.mu.RUnlock()
	timeoutCtx, cancel := context.WithTimeout(ctx, e.probeTimeout)
	defer cancel()
	select {
	case <-done:
		finished, _ := e.GetOrder(snapshot.ID)
		completed := finished.UpdatedAt
		result.CompletedAt = &completed
		if finished.Status == order.StatusCompleted {
			result.Result = "completed"
		} else {
			result.Result = "failed"
		}
	case <-timeoutCtx.Done():
		result.Result = "timed_out"
	}
	result.Duration = e.now().UTC().Sub(started)
	e.metrics.BusinessProbe(result.Result)
	return result
}

func (e *Engine) resizeLocked(desired int) {
	for len(e.workers) < desired {
		e.nextWorker++
		id := e.nextWorker
		ctx, cancel := context.WithCancel(context.Background())
		e.workers[id] = cancel
		e.active++
		go e.workerLoop(id, ctx)
	}
	for id, cancel := range e.workers {
		if len(e.workers) <= desired {
			break
		}
		delete(e.workers, id)
		cancel()
	}
}

func (e *Engine) workerLoop(id int, ctx context.Context) {
	defer func() {
		e.mu.Lock()
		if _, exists := e.workers[id]; exists {
			delete(e.workers, id)
		}
		e.active--
		e.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case orderID := <-e.queue:
			e.process(orderID)
		}
	}
}

func (e *Engine) process(id string) {
	started := e.now().UTC()
	e.mu.Lock()
	o, ok := e.orders[id]
	if !ok || o.Start(started) != nil {
		e.mu.Unlock()
		return
	}
	e.inflight++
	slow, dependency := e.faults[FaultSlowProcessing], e.faults[FaultDependencyErrors]
	e.mu.Unlock()

	delay := e.processingDelay
	if slow {
		delay = e.slowDelay
	}
	if dependency {
		for range 2 {
			time.Sleep(e.dependencyDelay)
			e.metrics.OrderRetried(FailureDependency)
		}
		delay = e.dependencyDelay
	}
	time.Sleep(delay)

	finished := e.now().UTC()
	e.mu.Lock()
	if dependency {
		_ = o.Fail(FailureRetryExhausted, finished)
	} else {
		_ = o.Complete(finished)
	}
	e.inflight--
	done := e.done[id]
	close(done)
	e.mu.Unlock()
	e.metrics.ObserveProcessing(finished.Sub(started))
	e.metrics.ObserveEndToEnd(finished.Sub(o.CreatedAt))
	if dependency {
		e.metrics.OrderFailed(FailureRetryExhausted)
		e.metrics.OrderCompleted("failed")
	} else {
		e.metrics.OrderCompleted("completed")
	}
}

func snapshotOrder(o order.Order) OrderSnapshot {
	return OrderSnapshot{ID: o.ID, Status: o.Status, FailureReason: o.FailureReason, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt}
}

func validFault(scenario string) bool {
	return scenario == FaultWorkerStopped || scenario == FaultSlowProcessing || scenario == FaultDependencyErrors
}

func digest(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func randomID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("random id: %v", err))
	}
	return hex.EncodeToString(value[:])
}
