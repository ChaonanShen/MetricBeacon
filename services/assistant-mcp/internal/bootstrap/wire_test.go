package bootstrap_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"mini-torchbearing.local/services/assistant-mcp/internal/bootstrap"
)

func TestStreamableHTTPMCPTools(t *testing.T) {
	runtime, err := bootstrap.Wire(bootstrap.Config{FixtureDir: repositoryPath(t, "data/mock-scenarios/node_exporter_overview"), SchemaDir: repositoryPath(t, "contracts/tools/grafana")})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(runtime.Handler)
	t.Cleanup(server.Close)

	for _, path := range []string{"/healthz", "/readyz"} {
		response, err := http.Get(server.URL + path)
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("%s is not healthy: %v", path, err)
		}
		response.Body.Close()
	}

	context := context.Background()
	client := newClient(t, server.URL+"/mcp", headers(true))
	initialize(t, context, client)
	tools, err := client.ListTools(context, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	wantNames := []string{"grafana.get_metric_labels", "grafana.query_prometheus", "grafana.search_metrics"}
	if len(names) != len(wantNames) {
		t.Fatalf("unexpected tools: %#v", names)
	}
	for index := range names {
		if names[index] != wantNames[index] {
			t.Fatalf("unexpected tools: %#v", names)
		}
	}

	search := call(t, context, client, "grafana.search_metrics", map[string]any{"datasourceUid": "mock-prometheus", "query": "node exporter", "limit": 10})
	if search.IsError {
		t.Fatalf("search returned an error: %#v", search)
	}
	var searchOutput struct {
		Candidates []any `json:"candidates"`
	}
	decodeStructured(t, search, &searchOutput)
	if len(searchOutput.Candidates) != 4 {
		t.Fatalf("unexpected search output: %#v", searchOutput)
	}

	labels := call(t, context, client, "grafana.get_metric_labels", map[string]any{"datasourceUid": "mock-prometheus", "metricName": "node_cpu_seconds_total"})
	if labels.IsError {
		t.Fatalf("labels returned an error: %#v", labels)
	}
	var labelsOutput struct {
		MetricName string   `json:"metricName"`
		LabelNames []string `json:"labelNames"`
	}
	decodeStructured(t, labels, &labelsOutput)
	if labelsOutput.MetricName != "node_cpu_seconds_total" || len(labelsOutput.LabelNames) != 4 {
		t.Fatalf("unexpected labels output: %#v", labelsOutput)
	}

	start := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	query := call(t, context, client, "grafana.query_prometheus", map[string]any{"datasourceUid": "mock-prometheus", "expression": `100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`, "start": start.Format(time.RFC3339), "end": start.Add(30 * time.Minute).Format(time.RFC3339), "stepSeconds": 300, "mode": "execute"})
	if query.IsError {
		t.Fatalf("query returned an error: %#v", query)
	}
	var queryOutput struct {
		Status string `json:"status"`
		Series []any  `json:"series"`
	}
	decodeStructured(t, query, &queryOutput)
	if queryOutput.Status != "success" || len(queryOutput.Series) != 2 {
		t.Fatalf("unexpected query output: %#v", queryOutput)
	}

	invalid := call(t, context, client, "grafana.search_metrics", map[string]any{"datasourceUid": "wrong", "query": "node exporter", "limit": 10})
	if !invalid.IsError {
		t.Fatal("invalid schema input unexpectedly succeeded")
	}
	unauthorizedClient := newClient(t, server.URL+"/mcp", headers(false))
	initialize(t, context, unauthorizedClient)
	unauthorized := call(t, context, unauthorizedClient, "grafana.search_metrics", map[string]any{"datasourceUid": "mock-prometheus", "query": "node exporter", "limit": 10})
	if !unauthorized.IsError {
		t.Fatal("request without datasources:query unexpectedly succeeded")
	}
}

func newClient(t *testing.T, endpoint string, headers map[string]string) *client.Client {
	t.Helper()
	transport, err := transport.NewStreamableHTTP(endpoint, transport.WithHTTPHeaders(headers))
	if err != nil {
		t.Fatal(err)
	}
	return client.NewClient(transport)
}

func initialize(t *testing.T, context context.Context, client *client.Client) {
	t.Helper()
	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = mcp.Implementation{Name: "assistant-mcp-contract-test", Version: "v1"}
	request.Params.Capabilities = mcp.ClientCapabilities{}
	if _, err := client.Initialize(context, request); err != nil {
		t.Fatal(err)
	}
}

func call(t *testing.T, context context.Context, client *client.Client, name string, arguments map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := client.CallTool(context, mcp.CallToolRequest{Params: mcp.CallToolParams{Name: name, Arguments: arguments}})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func decodeStructured(t *testing.T, result *mcp.CallToolResult, destination any) {
	t.Helper()
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, destination); err != nil {
		t.Fatal(err)
	}
}

func headers(withPermission bool) map[string]string {
	permissions := ""
	if withPermission {
		permissions = "datasources:query"
	}
	return map[string]string{
		"X-MTB-Tenant-ID":   "org:1",
		"X-MTB-Org-ID":      "1",
		"X-MTB-User-ID":     "user:1",
		"X-MTB-Permissions": permissions,
		"X-Request-ID":      "request-test",
		"X-Trace-ID":        "trace-test",
	}
}

func repositoryPath(t *testing.T, relative string) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(directory, relative)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatalf("repository path %q was not found", relative)
		}
		directory = parent
	}
}
