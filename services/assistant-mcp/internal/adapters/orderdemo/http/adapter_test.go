package http_test

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	orderhttp "mini-torchbearing.local/services/assistant-mcp/internal/adapters/orderdemo/http"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/orderdemo"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

func TestHTTPAdapterReadContractAndAuthorization(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	seen := map[string]int{}
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
		if request.Header.Get("Authorization") != "Bearer read-secret" {
			t.Fatalf("missing read credential on %s", request.URL.Path)
		}
		seen[request.URL.Path]++
		switch request.URL.Path {
		case "/ops/v1/runtime":
			writeJSON(t, writer, map[string]any{"serviceRef": "order-demo", "instanceEpoch": "epoch-1", "startedAt": now.Add(-time.Hour), "supervisorStatus": "running"})
		case "/ops/v1/queue":
			writeJSON(t, writer, map[string]any{"depth": 7, "capacity": 100, "oldestAgeSeconds": 12.5, "observedAt": now})
		case "/ops/v1/config/worker":
			writeJSON(t, writer, map[string]any{"serviceRef": "order-demo", "instanceEpoch": "epoch-1", "configuredConcurrency": 0, "effectiveConcurrency": 0, "activeWorkers": 0, "inflightOrders": 0, "version": 2, "observedAt": now})
		case "/ops/v1/policies/worker":
			writeJSON(t, writer, map[string]any{"serviceRef": "order-demo", "expectedConcurrency": 2, "minConcurrency": 1, "maxConcurrency": 4, "version": "v1", "digest": strings.Repeat("a", 64)})
		case "/ops/v1/orders/recent":
			if request.URL.Query().Get("limit") != "3" || request.URL.Query().Get("status") != "failed" {
				t.Fatalf("unexpected bounded query: %s", request.URL.RawQuery)
			}
			writeJSON(t, writer, map[string]any{"orders": []map[string]any{{"id": "redacted-1", "status": "failed", "createdAt": now.Add(-time.Minute), "updatedAt": now, "failureReason": "retry_exhausted"}}})
		case "/ops/v1/operations/op-1":
			writeJSON(t, writer, map[string]any{"operationId": "op-1", "instanceEpoch": "epoch-1", "beforeVersion": 2, "afterVersion": 3, "beforeConcurrency": 0, "afterConcurrency": 2, "intentDigest": "sha256:" + strings.Repeat("b", 64), "approvalId": "approval-1", "executedAt": now})
		default:
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	adapter, err := orderhttp.New(server.URL, "read-secret", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	identity := requestcontext.Context{TenantID: "org:1"}
	runtimeState, err := adapter.GetRuntime(ctx, identity)
	if err != nil || runtimeState.InstanceEpoch != "epoch-1" {
		t.Fatalf("runtime: %#v %v", runtimeState, err)
	}
	queue, err := adapter.GetQueue(ctx, identity)
	if err != nil || queue.Depth != 7 || queue.Capacity != 100 {
		t.Fatalf("queue: %#v %v", queue, err)
	}
	worker, err := adapter.GetWorker(ctx, identity)
	if err != nil || worker.Version != 2 || worker.ConfiguredConcurrency != 0 {
		t.Fatalf("worker: %#v %v", worker, err)
	}
	policy, err := adapter.GetPolicy(ctx, identity)
	if err != nil || policy.ExpectedConcurrency != 2 || policy.Version != "v1" {
		t.Fatalf("policy: %#v %v", policy, err)
	}
	outcomes, err := adapter.GetRecentOutcomes(ctx, identity, orderdemo.RecentRequest{Status: "failed", Limit: 3})
	if err != nil || len(outcomes) != 1 || outcomes[0].FailureReason == nil || *outcomes[0].FailureReason != "retry_exhausted" {
		t.Fatalf("outcomes: %#v %v", outcomes, err)
	}
	operation, err := adapter.GetOperation(ctx, identity, "op-1")
	if err != nil || operation.BeforeConcurrency != 0 || operation.AfterConcurrency != 2 || operation.AfterVersion != 3 {
		t.Fatalf("operation: %#v %v", operation, err)
	}
	if len(seen) != 6 {
		t.Fatalf("not all typed endpoints were called: %#v", seen)
	}
}

func TestHTTPAdapterSeparatesRemediationCredentialAndRunsFixedProbe(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	digest := "sha256:" + strings.Repeat("c", 64)
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
		switch request.URL.Path {
		case "/ops/v1/config/worker":
			if request.Method != stdhttp.MethodPut || request.Header.Get("Authorization") != "Bearer write-secret" {
				t.Fatalf("write credential boundary failed: %s %s", request.Method, request.Header.Get("Authorization"))
			}
			writeJSON(t, writer, map[string]any{"operationId": "op-1", "instanceEpoch": "epoch-1", "beforeVersion": 2, "afterVersion": 3, "beforeConcurrency": 0, "afterConcurrency": 2, "intentDigest": digest, "approvalId": "approval-1", "executedAt": now})
		case "/ops/v1/probes/order-processing":
			if request.Method != stdhttp.MethodPost || request.Header.Get("Authorization") != "Bearer read-secret" {
				t.Fatalf("probe credential boundary failed: %s %s", request.Method, request.Header.Get("Authorization"))
			}
			writeJSON(t, writer, map[string]any{"probeId": "probe-1", "orderId": "must-be-redacted-by-adapter", "result": "completed", "durationMs": 203, "completedAt": now})
		default:
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	adapter, err := orderhttp.NewWithRemediation(server.URL, "read-secret", "write-secret", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	request := orderdemo.RemediationRequest{OperationID: "op-1", InstanceEpoch: "epoch-1", ExpectedVersion: 2, ExpectedConcurrency: 0, NewConcurrency: 2, IntentDigest: digest, ApprovalID: "approval-1"}
	receipt, err := adapter.RestoreWorkerConcurrency(context.Background(), requestcontext.Context{}, request)
	if err != nil || receipt.AfterVersion != 3 || receipt.IntentDigest != digest {
		t.Fatalf("receipt=%#v err=%v", receipt, err)
	}
	probe, err := adapter.RunBusinessProbe(context.Background(), requestcontext.Context{}, "probe-1")
	if err != nil || probe.Result != "completed" || probe.DurationMS != 203 || probe.CompletedAt == nil {
		t.Fatalf("probe=%#v err=%v", probe, err)
	}
}

func TestHTTPAdapterRejectsInvalidConfigurationAndInputs(t *testing.T) {
	for _, test := range []struct {
		endpoint, token string
		timeout         time.Duration
	}{{"", "token", time.Second}, {"order:8090", "token", time.Second}, {"ftp://order", "token", time.Second}, {"http://order", "", time.Second}, {"http://order", "token", 0}} {
		if _, err := orderhttp.New(test.endpoint, test.token, test.timeout); err == nil {
			t.Fatalf("invalid config accepted: %#v", test)
		}
	}
	if _, err := orderhttp.NewWithRemediation("http://order", "read", "", time.Second); err == nil {
		t.Fatal("empty remediation credential was accepted")
	}
	server := httptest.NewServer(stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) { t.Fatal("invalid input contacted service") }))
	t.Cleanup(server.Close)
	adapter, _ := orderhttp.New(server.URL, "token", time.Second)
	for _, request := range []orderdemo.RecentRequest{{Limit: 0}, {Limit: 21}, {Limit: 1, Status: "unknown"}} {
		_, err := adapter.GetRecentOutcomes(context.Background(), requestcontext.Context{}, request)
		requireCode(t, err, runtime.SchemaValidationFailed)
	}
	_, err := adapter.GetOperation(context.Background(), requestcontext.Context{}, "")
	requireCode(t, err, runtime.SchemaValidationFailed)
	_, err = adapter.RestoreWorkerConcurrency(context.Background(), requestcontext.Context{}, orderdemo.RemediationRequest{})
	requireCode(t, err, runtime.AdapterNotConfigured)
}

func TestHTTPAdapterClassifiesFailuresAndRejectsUnboundedResponses(t *testing.T) {
	tests := []struct {
		name    string
		handler stdhttp.HandlerFunc
		timeout time.Duration
		want    runtime.ErrorCode
	}{
		{"unauthorized", func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
			writer.WriteHeader(stdhttp.StatusUnauthorized)
		}, time.Second, runtime.AdapterNotConfigured},
		{"not-found", func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) { writer.WriteHeader(stdhttp.StatusNotFound) }, time.Second, runtime.ResourceNotFound},
		{"malformed", func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) { _, _ = writer.Write([]byte("not-json")) }, time.Second, runtime.DependencyUnavailable},
		{"oversized", func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
			_, _ = writer.Write([]byte(strings.Repeat("x", 64<<10+1)))
		}, time.Second, runtime.SchemaValidationFailed},
		{"timeout", func(_ stdhttp.ResponseWriter, _ *stdhttp.Request) { time.Sleep(100 * time.Millisecond) }, 10 * time.Millisecond, runtime.ToolTimeout},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			t.Cleanup(server.Close)
			adapter, _ := orderhttp.New(server.URL, "token", test.timeout)
			_, err := adapter.GetRuntime(context.Background(), requestcontext.Context{})
			requireCode(t, err, test.want)
		})
	}
}

func TestHTTPAdapterRejectsRedirectAndSemanticallyInvalidPayload(t *testing.T) {
	target := httptest.NewServer(stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) { t.Fatal("redirect followed") }))
	t.Cleanup(target.Close)
	redirect := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
		writer.Header().Set("Location", target.URL)
		writer.WriteHeader(stdhttp.StatusFound)
	}))
	t.Cleanup(redirect.Close)
	adapter, _ := orderhttp.New(redirect.URL, "token", time.Second)
	_, err := adapter.GetRuntime(context.Background(), requestcontext.Context{})
	requireCode(t, err, runtime.DependencyUnavailable)

	invalid := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
		writeJSON(t, writer, map[string]any{"depth": 101, "capacity": 100, "oldestAgeSeconds": 1, "observedAt": time.Now().UTC()})
	}))
	t.Cleanup(invalid.Close)
	adapter, _ = orderhttp.New(invalid.URL, "token", time.Second)
	_, err = adapter.GetQueue(context.Background(), requestcontext.Context{})
	requireCode(t, err, runtime.SchemaValidationFailed)
}

func writeJSON(t *testing.T, writer stdhttp.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func requireCode(t *testing.T, err error, want runtime.ErrorCode) {
	t.Helper()
	var toolError *runtime.ToolError
	if !errors.As(err, &toolError) || toolError.Code != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
