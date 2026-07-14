package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	protocol "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

func TestGatewayAndTypedAdaptersUseRealStreamableHTTP(t *testing.T) {
	server, seen := testServer(t)
	defer server.Close()
	identity := requestcontext.Context{TenantID: "org:1", OrgID: "1", UserID: "user:1", Roles: []string{"Viewer"}, Permissions: []string{"datasources:query"}, RequestID: "request-one", TraceID: "trace-one"}
	gateway := NewGateway(server.URL+"/mcp", time.Second)
	descriptors, err := gateway.ListTools(context.Background(), identity, tools.Filter{Namespace: "grafana"})
	if err != nil || len(descriptors) != 3 {
		t.Fatalf("list tools: %#v, %v", descriptors, err)
	}
	catalog := NewMetricCatalogAdapter(gateway)
	metrics, err := catalog.SearchMetrics(context.Background(), identity, dto.SearchMetricsRequest{DatasourceUID: "prometheus-main", Query: "node", Limit: 10})
	if err != nil || len(metrics.Candidates) != 3 {
		t.Fatalf("search metrics: %#v, %v", metrics, err)
	}
	labels, err := catalog.GetMetricLabels(context.Background(), identity, dto.GetMetricLabelsRequest{DatasourceUID: "prometheus-main", MetricName: "node_cpu_seconds_total"})
	if err != nil || len(labels.LabelNames) != 1 || labels.LabelNames[0] != "instance" {
		t.Fatalf("labels: %#v, %v", labels, err)
	}
	query := NewQueryEngineAdapter(gateway)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	result, err := query.Execute(context.Background(), identity, dto.ExecuteQueryRequest{DatasourceUID: "prometheus-main", Expression: "node_load1", TimeRange: common.AbsoluteTimeRange{From: now.Add(-time.Minute), To: now}, StepSeconds: 60})
	if err != nil || result.Status != "success" || len(result.Series) != 1 {
		t.Fatalf("query: %#v, %v", result, err)
	}
	if seen.get("request-one") == 0 {
		t.Fatal("per-request identity headers did not reach MCP")
	}
}

func TestGatewayKeepsConcurrentRequestHeadersIsolated(t *testing.T) {
	server, seen := testServer(t)
	defer server.Close()
	gateway := NewGateway(server.URL+"/mcp", time.Second)
	var group sync.WaitGroup
	for _, requestID := range []string{"request-a", "request-b"} {
		group.Add(1)
		go func(requestID string) {
			defer group.Done()
			_, err := gateway.CallTool(context.Background(), requestcontext.Context{TenantID: "org:1", OrgID: "1", UserID: requestID, Permissions: []string{"datasources:query"}, RequestID: requestID, TraceID: requestID}, tools.Call{Name: "grafana.search_metrics", Version: "v1", Arguments: json.RawMessage(`{"datasourceUid":"prometheus-main","query":"node","limit":10}`)})
			if err != nil {
				t.Errorf("call %s: %v", requestID, err)
			}
		}(requestID)
	}
	group.Wait()
	if seen.get("request-a") == 0 || seen.get("request-b") == 0 {
		t.Fatalf("headers were mixed or missing: %#v", seen.values)
	}
}

type requestsSeen struct {
	sync.Mutex
	values map[string]int
}

func (s *requestsSeen) add(id string)     { s.Lock(); defer s.Unlock(); s.values[id]++ }
func (s *requestsSeen) get(id string) int { s.Lock(); defer s.Unlock(); return s.values[id] }

func testServer(t *testing.T) (*httptest.Server, *requestsSeen) {
	t.Helper()
	seen := &requestsSeen{values: map[string]int{}}
	mcpServer := server.NewMCPServer("test", "v1")
	for _, registration := range []struct {
		name   string
		output any
	}{
		{"grafana.search_metrics", map[string]any{"candidates": []map[string]any{{"metricName": "node_cpu_seconds_total"}, {"metricName": "node_memory_MemAvailable_bytes"}, {"metricName": "node_load1"}}}},
		{"grafana.get_metric_labels", map[string]any{"metricName": "node_cpu_seconds_total", "labelNames": []string{"instance"}, "sampleValues": map[string][]string{"instance": {"node-a"}}}},
		{"grafana.query_prometheus", map[string]any{"validation": map[string]any{"valid": true, "canonicalExpression": "node_load1"}, "status": "success", "resultType": "matrix", "series": []map[string]any{{"name": "node-a", "labels": map[string]string{"instance": "node-a"}, "points": []map[string]any{{"timestamp": "2026-07-13T12:00:00Z", "value": 1.0}}}}, "durationMs": 1, "warnings": []string{}}},
	} {
		output := registration.output
		mcpServer.AddTool(protocol.NewTool(registration.name), func(_ context.Context, request protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			seen.add(request.Header.Get("X-Request-ID"))
			return protocol.NewToolResultStructuredOnly(output), nil
		})
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true)))
	return httptest.NewServer(mux), seen
}
