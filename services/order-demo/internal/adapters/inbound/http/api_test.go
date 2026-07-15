package httpadapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"mini-torchbearing.local/services/order-demo/internal/adapters/inbound/http/generated"
	"mini-torchbearing.local/services/order-demo/internal/application"
)

func newTestAPI(t *testing.T) (*API, *application.Engine) {
	t.Helper()
	var id atomic.Int64
	engine := application.NewEngine(application.Options{ProcessingDelay: 5 * time.Millisecond, ProbeTimeout: 100 * time.Millisecond,
		ID: func() string { return fmt.Sprintf("id-%d", id.Add(1)) }})
	t.Cleanup(engine.Close)
	return NewAPI(engine), engine
}

func TestBusinessAndOperationalSurfacesAreIsolated(t *testing.T) {
	api, _ := newTestAPI(t)
	business := api.BusinessHandler()
	ops := api.OperationalHandler("read-secret", "write-secret")

	assertStatus(t, business, request(http.MethodGet, "/ops/v1/config/worker", "", ""), http.StatusNotFound)
	assertStatus(t, business, request(http.MethodPost, "/faults/v1/reset", "", ""), http.StatusNotFound)
	assertStatus(t, ops, request(http.MethodPost, "/api/v1/orders", `{}`, "read-secret"), http.StatusNotFound)
	assertStatus(t, ops, request(http.MethodGet, "/ops/v1/config/worker", "", ""), http.StatusUnauthorized)
	assertStatus(t, ops, request(http.MethodGet, "/ops/v1/config/worker", "", "write-secret"), http.StatusUnauthorized)
	assertStatus(t, ops, request(http.MethodGet, "/ops/v1/config/worker", "", "read-secret"), http.StatusOK)
}

func TestBusinessOrderAndStrictJSON(t *testing.T) {
	api, _ := newTestAPI(t)
	handler := api.BusinessHandler()
	req := request(http.MethodPost, "/api/v1/orders", `{"sku":"DEMO","quantity":1}`, "")
	req.Header.Set("Idempotency-Key", "business-1")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var created generated.Order
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, handler, request(http.MethodGet, "/api/v1/orders/"+created.Id, "", ""), http.StatusOK)

	unknown := request(http.MethodPost, "/api/v1/orders", `{"sku":"DEMO","quantity":1,"fault":true}`, "")
	unknown.Header.Set("Idempotency-Key", "business-2")
	assertStatus(t, handler, unknown, http.StatusBadRequest)
}

func TestFaultAndRemediationUseDifferentCapabilities(t *testing.T) {
	api, engine := newTestAPI(t)
	fault := api.FaultHandler()
	ops := api.OperationalHandler("read-secret", "write-secret")
	assertStatus(t, fault, request(http.MethodPost, "/faults/v1/scenarios/worker-stopped/activate", "", ""), http.StatusOK)
	waitHTTP(t, func() bool { return engine.WorkerSnapshot().ActiveWorkers == 0 })
	stopped := engine.WorkerSnapshot()

	body := generated.UpdateWorkerConfigRequest{OperationId: "op-1", InstanceEpoch: stopped.InstanceEpoch, ExpectedVersion: stopped.Version,
		ExpectedConcurrency: 0, NewConcurrency: 2, IntentDigest: strings.Repeat("a", 64), ApprovalId: "approval-1"}
	encoded, _ := json.Marshal(body)
	assertStatus(t, ops, request(http.MethodPut, "/ops/v1/config/worker", string(encoded), "read-secret"), http.StatusUnauthorized)
	assertStatus(t, ops, request(http.MethodPut, "/ops/v1/config/worker", string(encoded), "write-secret"), http.StatusOK)
	waitHTTP(t, func() bool { return engine.WorkerSnapshot().ActiveWorkers == 2 })
	assertStatus(t, ops, request(http.MethodPost, "/faults/v1/reset", "", "write-secret"), http.StatusNotFound)
}

func TestStaleRemediationDoesNotWrite(t *testing.T) {
	api, engine := newTestAPI(t)
	_ = engine.ActivateFault(application.FaultWorkerStopped)
	waitHTTP(t, func() bool { return engine.WorkerSnapshot().ActiveWorkers == 0 })
	stopped := engine.WorkerSnapshot()
	body := generated.UpdateWorkerConfigRequest{OperationId: "op-stale", InstanceEpoch: stopped.InstanceEpoch, ExpectedVersion: stopped.Version + 1,
		ExpectedConcurrency: 0, NewConcurrency: 2, IntentDigest: strings.Repeat("b", 64), ApprovalId: "approval-1"}
	encoded, _ := json.Marshal(body)
	assertStatus(t, api.OperationalHandler("read-secret", "write-secret"), request(http.MethodPut, "/ops/v1/config/worker", string(encoded), "write-secret"), http.StatusConflict)
	if got := engine.WorkerSnapshot().ConfiguredConcurrency; got != 0 {
		t.Fatalf("configured concurrency = %d", got)
	}
}

func request(method, path, body, token string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func assertStatus(t *testing.T, handler http.Handler, req *http.Request, wanted int) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != wanted {
		t.Fatalf("%s %s status = %d want %d body=%s", req.Method, req.URL.Path, recorder.Code, wanted, recorder.Body.String())
	}
}

func waitHTTP(t *testing.T, predicate func() bool) {
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
