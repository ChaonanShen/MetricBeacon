package httpadapter

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"mini-torchbearing.local/services/order-demo/internal/adapters/inbound/http/faultgenerated"
	"mini-torchbearing.local/services/order-demo/internal/adapters/inbound/http/generated"
	"mini-torchbearing.local/services/order-demo/internal/application"
	"mini-torchbearing.local/services/order-demo/internal/domain/order"
	"mini-torchbearing.local/services/order-demo/internal/domain/worker"
)

type API struct {
	engine  *application.Engine
	now     func() time.Time
	mu      sync.Mutex
	changed map[string]time.Time
}

func NewAPI(engine *application.Engine) *API {
	return &API{engine: engine, now: time.Now, changed: make(map[string]time.Time)}
}

func (a *API) BusinessHandler() http.Handler {
	generatedHandler := generated.Handler(a)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" && !strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		generatedHandler.ServeHTTP(w, r)
	})
}

func (a *API) OperationalHandler(readToken, remediationToken string) http.Handler {
	generatedHandler := generated.Handler(a)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/ops/") {
			http.NotFound(w, r)
			return
		}
		expected := readToken
		if r.Method == http.MethodPut {
			expected = remediationToken
		}
		if !bearerMatches(r.Header.Get("Authorization"), expected) {
			writeError(w, http.StatusUnauthorized, generated.Unauthenticated, "invalid operational credential", false)
			return
		}
		generatedHandler.ServeHTTP(w, r)
	})
}

func (a *API) FaultHandler() http.Handler { return faultgenerated.Handler(a) }

func (a *API) Healthz(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
func (a *API) Readyz(w http.ResponseWriter, _ *http.Request)  { w.WriteHeader(http.StatusOK) }

func (a *API) CreateOrder(w http.ResponseWriter, r *http.Request, params generated.CreateOrderParams) {
	var body generated.CreateOrderRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, generated.InvalidArgument, "invalid order request", false)
		return
	}
	created, err := a.engine.Submit(params.IdempotencyKey, body.Sku, body.Quantity)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, toWireOrder(created))
}

func (a *API) GetOrder(w http.ResponseWriter, _ *http.Request, orderID generated.OrderId) {
	item, err := a.engine.GetOrder(orderID)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWireOrder(item))
}

func (a *API) GetRuntime(w http.ResponseWriter, _ *http.Request) {
	workers := a.engine.WorkerSnapshot()
	status := generated.Running
	if workers.ActiveWorkers != workers.ConfiguredConcurrency {
		status = generated.Degraded
	}
	writeJSON(w, http.StatusOK, generated.RuntimeSnapshot{ServiceRef: application.ServiceRef, InstanceEpoch: workers.InstanceEpoch, StartedAt: a.engine.StartedAt(), SupervisorStatus: status})
}

func (a *API) GetQueue(w http.ResponseWriter, _ *http.Request) {
	queue := a.engine.QueueSnapshot()
	writeJSON(w, http.StatusOK, generated.QueueSnapshot{Depth: queue.Depth, Capacity: queue.Capacity, OldestAgeSeconds: float32(queue.OldestAgeSeconds), ObservedAt: queue.ObservedAt})
}

func (a *API) GetWorkerConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, toWireWorker(a.engine.WorkerSnapshot()))
}

func (a *API) GetWorkerPolicy(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, generated.WorkerPolicy{ServiceRef: application.ServiceRef, ExpectedConcurrency: worker.ExpectedConcurrency,
		MinConcurrency: worker.MinConcurrency, MaxConcurrency: worker.MaxConcurrency, Version: "v1", Digest: worker.PolicyDigest()})
}

func (a *API) ListRecentOrders(w http.ResponseWriter, _ *http.Request, params generated.ListRecentOrdersParams) {
	status := order.Status("")
	if params.Status != nil {
		status = order.Status(*params.Status)
	}
	limit := 10
	if params.Limit != nil {
		limit = *params.Limit
	}
	items := a.engine.Recent(status, limit)
	result := make([]generated.Order, 0, len(items))
	for _, item := range items {
		result = append(result, toWireOrder(item))
	}
	writeJSON(w, http.StatusOK, struct {
		Orders []generated.Order `json:"orders"`
	}{Orders: result})
}

func (a *API) UpdateWorkerConfig(w http.ResponseWriter, r *http.Request) {
	var body generated.UpdateWorkerConfigRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, generated.InvalidArgument, "invalid remediation request", false)
		return
	}
	expected, expectedOK := integer(body.ExpectedConcurrency)
	newValue, newOK := integer(body.NewConcurrency)
	if !expectedOK || !newOK {
		writeError(w, http.StatusBadRequest, generated.InvalidArgument, "concurrency must be an integer", false)
		return
	}
	receipt, err := a.engine.Remediate(application.RemediationRequest{OperationID: body.OperationId, InstanceEpoch: body.InstanceEpoch,
		ExpectedVersion: body.ExpectedVersion, ExpectedConcurrency: expected, NewConcurrency: newValue, IntentDigest: body.IntentDigest, ApprovalID: body.ApprovalId})
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWireReceipt(receipt))
}

func (a *API) GetOperation(w http.ResponseWriter, _ *http.Request, operationID generated.OperationId) {
	receipt, err := a.engine.GetOperation(operationID)
	if err != nil {
		writeError(w, http.StatusNotFound, generated.OperationNotFound, "operation not found", false)
		return
	}
	writeJSON(w, http.StatusOK, toWireReceipt(receipt))
}

func (a *API) RunOrderProcessingProbe(w http.ResponseWriter, r *http.Request) {
	var body generated.RunOrderProcessingProbeJSONBody
	if err := decodeJSON(r, &body); err != nil || body.ProbeId == "" {
		writeError(w, http.StatusBadRequest, generated.InvalidArgument, "invalid probe request", false)
		return
	}
	result := a.engine.RunProbe(r.Context(), body.ProbeId)
	wireResult := generated.ProbeResult{ProbeId: result.ProbeID, OrderId: result.OrderID, Result: generated.ProbeResultResult(result.Result), DurationMs: int(result.Duration.Milliseconds()), CompletedAt: result.CompletedAt}
	writeJSON(w, http.StatusOK, wireResult)
}

func (a *API) ActivateFault(w http.ResponseWriter, _ *http.Request, scenario faultgenerated.ActivateFaultParamsScenario) {
	value := string(scenario)
	if err := a.engine.ActivateFault(value); err != nil {
		writeFaultError(w, http.StatusBadRequest, "invalid_scenario", "unknown fault scenario")
		return
	}
	a.mu.Lock()
	a.changed[value] = a.now().UTC()
	changed := a.changed[value]
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, faultgenerated.FaultState{Scenario: faultgenerated.FaultStateScenario(value), Active: true, ChangedAt: changed})
}

func (a *API) ClearFault(w http.ResponseWriter, _ *http.Request, scenario faultgenerated.ClearFaultParamsScenario) {
	value := string(scenario)
	if err := a.engine.ClearFault(value); err != nil {
		writeFaultError(w, http.StatusBadRequest, "invalid_scenario", "unknown fault scenario")
		return
	}
	changed := a.now().UTC()
	a.mu.Lock()
	a.changed[value] = changed
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, faultgenerated.FaultState{Scenario: faultgenerated.FaultStateScenario(value), Active: false, ChangedAt: changed})
}

func (a *API) ResetFaults(w http.ResponseWriter, _ *http.Request) {
	a.engine.ResetFaults()
	a.mu.Lock()
	a.changed = make(map[string]time.Time)
	a.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func bearerMatches(header, expected string) bool {
	if expected == "" || !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	actual := strings.TrimPrefix(header, "Bearer ")
	return len(actual) == len(expected) && subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func integer(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), number == float64(int(number))
	case int:
		return number, true
	default:
		return 0, false
	}
}

func toWireOrder(value application.OrderSnapshot) generated.Order {
	result := generated.Order{Id: value.ID, Status: generated.OrderStatus(value.Status), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	if value.FailureReason != "" {
		reason := generated.OrderFailureReason(value.FailureReason)
		result.FailureReason = &reason
	}
	return result
}

func toWireWorker(value application.WorkerSnapshot) generated.WorkerConfig {
	return generated.WorkerConfig{ServiceRef: value.ServiceRef, InstanceEpoch: value.InstanceEpoch, ConfiguredConcurrency: value.ConfiguredConcurrency,
		EffectiveConcurrency: value.EffectiveConcurrency, ActiveWorkers: value.ActiveWorkers, InflightOrders: value.InflightOrders, Version: value.Version, ObservedAt: value.ObservedAt}
}

func toWireReceipt(value application.OperationReceipt) generated.OperationReceipt {
	return generated.OperationReceipt{OperationId: value.OperationID, InstanceEpoch: value.InstanceEpoch, BeforeVersion: value.BeforeVersion,
		AfterVersion: value.AfterVersion, BeforeConcurrency: value.BeforeConcurrency, AfterConcurrency: value.AfterConcurrency,
		IntentDigest: value.IntentDigest, ApprovalId: value.ApprovalID, ExecutedAt: value.ExecutedAt}
}

func writeApplicationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrInvalidArgument):
		writeError(w, http.StatusBadRequest, generated.InvalidArgument, "invalid request", false)
	case errors.Is(err, application.ErrQueueFull):
		writeError(w, http.StatusServiceUnavailable, generated.QueueFull, "order queue is full", true)
	case errors.Is(err, application.ErrNotFound):
		writeError(w, http.StatusNotFound, generated.ResourceNotFound, "resource not found", false)
	case errors.Is(err, application.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, generated.IdempotencyConflict, "idempotency key conflicts with prior request", false)
	case errors.Is(err, application.ErrPreconditionFailed):
		writeError(w, http.StatusConflict, generated.TargetPreconditionFailed, "remediation target changed", false)
	default:
		writeError(w, http.StatusInternalServerError, generated.InternalError, "internal error", false)
	}
}

func writeError(w http.ResponseWriter, status int, code generated.ErrorCode, message string, retryable bool) {
	writeJSON(w, status, generated.Error{Code: code, Message: message, Retryable: retryable})
}

func writeFaultError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, faultgenerated.Error{Code: faultgenerated.ErrorCode(code), Message: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
