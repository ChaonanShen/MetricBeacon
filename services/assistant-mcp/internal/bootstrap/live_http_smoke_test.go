package bootstrap_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestLivePrometheusMCPDiagnostic(t *testing.T) {
	if os.Getenv("MTB_RUN_LIVE_MCP_DIAGNOSTIC") != "1" {
		t.Skip("set MTB_RUN_LIVE_MCP_DIAGNOSTIC=1 to call a live assistant-mcp")
	}
	endpoint := os.Getenv("MTB_LIVE_MCP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8081/mcp"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := newClient(t, endpoint, headers(true))
	initialize(t, ctx, client)

	search := call(t, ctx, client, "grafana.search_metrics", map[string]any{
		"datasourceUid": "prometheus-main",
		"query":         "node exporter cpu memory load",
		"limit":         10,
	})
	if search.IsError {
		t.Fatal("[mcp] search_metrics returned an error")
	}
	var searchOutput struct {
		Candidates []json.RawMessage `json:"candidates"`
	}
	decodeStructured(t, search, &searchOutput)
	if len(searchOutput.Candidates) != 4 {
		t.Fatalf("[mcp] search_metrics candidates=%d, want 4", len(searchOutput.Candidates))
	}
	t.Logf("[mcp] search_metrics candidates=%d", len(searchOutput.Candidates))

	labels := call(t, ctx, client, "grafana.get_metric_labels", map[string]any{
		"datasourceUid": "prometheus-main",
		"metricName":    "node_cpu_seconds_total",
	})
	if labels.IsError {
		t.Fatal("[mcp] get_metric_labels returned an error")
	}
	var labelsOutput struct {
		LabelNames []string `json:"labelNames"`
	}
	decodeStructured(t, labels, &labelsOutput)
	if !contains(labelsOutput.LabelNames, "instance") || !contains(labelsOutput.LabelNames, "mode") {
		t.Fatalf("[mcp] CPU labels did not include instance and mode: %v", labelsOutput.LabelNames)
	}
	t.Logf("[mcp] get_metric_labels labels=%d", len(labelsOutput.LabelNames))

	end := time.Now().UTC().Truncate(time.Second)
	start := end.Add(-30 * time.Minute)
	queries := []struct {
		view       string
		expression string
	}{
		{view: "cpu", expression: `100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`},
		{view: "memory", expression: `100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes`},
		{view: "load", expression: `node_load1`},
	}
	for _, item := range queries {
		result := call(t, ctx, client, "grafana.query_prometheus", map[string]any{
			"datasourceUid": "prometheus-main",
			"expression":    item.expression,
			"start":         start.Format(time.RFC3339),
			"end":           end.Format(time.RFC3339),
			"stepSeconds":   300,
			"mode":          "execute",
		})
		if result.IsError {
			t.Fatalf("[mcp] query_prometheus view=%s returned an error", item.view)
		}
		var output struct {
			Status     string `json:"status"`
			ResultType string `json:"resultType"`
			Series     []struct {
				Points []json.RawMessage `json:"points"`
			} `json:"series"`
		}
		decodeStructured(t, result, &output)
		if output.Status != "success" || output.ResultType != "matrix" || len(output.Series) == 0 {
			t.Fatalf("[mcp] query_prometheus view=%s status=%s resultType=%s series=%d", item.view, output.Status, output.ResultType, len(output.Series))
		}
		samples := 0
		for _, series := range output.Series {
			if len(series.Points) == 0 {
				t.Fatalf("[mcp] query_prometheus view=%s returned an empty series", item.view)
			}
			samples += len(series.Points)
		}
		t.Logf("[mcp] query_prometheus view=%s resultType=%s series=%d samples=%d", item.view, output.ResultType, len(output.Series), samples)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
