package mock_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	mock "mini-torchbearing.local/services/assistant-mcp/internal/adapters/prometheus/mock"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/prometheus"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

func TestAdapterContract(t *testing.T) {
	adapter, err := mock.New(fixtureDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	identity := requestcontext.Context{TenantID: "org:1", OrgID: "1", UserID: "user:1", Permissions: []string{"datasources:query"}}
	search, err := adapter.SearchMetrics(context.Background(), identity, prometheus.SearchMetricsRequest{DatasourceUID: "prometheus-main", Query: "node exporter", Limit: 10})
	if err != nil || len(search.Candidates) != 4 {
		t.Fatalf("unexpected search result: %#v, %v", search, err)
	}
	labels, err := adapter.GetMetricLabels(context.Background(), identity, prometheus.GetMetricLabelsRequest{DatasourceUID: "prometheus-main", MetricName: "node_cpu_seconds_total"})
	if err != nil || len(labels.LabelNames) != 4 {
		t.Fatalf("unexpected labels result: %#v, %v", labels, err)
	}
	start := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	cpuWindow := 30
	query, err := adapter.Query(context.Background(), identity, prometheus.QueryRequest{DatasourceUID: "prometheus-main", View: "cpu", CPURateWindowSeconds: &cpuWindow, Start: start, End: start.Add(30 * time.Second), StepSeconds: 5, Mode: prometheus.ModeExecute})
	if err != nil || len(query.Series) != 2 || len(query.Series[0].Points) != 7 || !query.Series[0].Points[0].Timestamp.Equal(start) || !query.Series[0].Points[6].Timestamp.Equal(start.Add(30*time.Second)) || query.Validation.CanonicalExpression == mock.CPUQuery {
		t.Fatalf("unexpected query result: %#v, %v", query, err)
	}
	validated, err := adapter.Query(context.Background(), identity, prometheus.QueryRequest{DatasourceUID: "prometheus-main", View: "cpu", CPURateWindowSeconds: &cpuWindow, Start: start, End: start.Add(30 * time.Second), StepSeconds: 5, Mode: prometheus.ModeValidate})
	if err != nil || len(validated.Series) != 0 || !validated.Validation.Valid {
		t.Fatalf("unexpected validation result: %#v, %v", validated, err)
	}
	_, err = adapter.Query(context.Background(), identity, prometheus.QueryRequest{DatasourceUID: "prometheus-main", View: "unknown", Start: start, End: start.Add(time.Minute), StepSeconds: 300, Mode: prometheus.ModeExecute})
	requireCode(t, err, runtime.SchemaValidationFailed)
}

func TestInvalidFixtureFailsAtConstruction(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "manifest.yaml"), []byte("scenarioId: node_exporter_overview\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "search_metrics.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := mock.New(directory)
	requireCode(t, err, runtime.SchemaValidationFailed)
}

func fixtureDirectory(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(directory, "data/mock-scenarios/node_exporter_overview")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("repository fixture directory was not found")
		}
		directory = parent
	}
}

func requireCode(t *testing.T, err error, want runtime.ErrorCode) {
	t.Helper()
	var toolError *runtime.ToolError
	if !errors.As(err, &toolError) || toolError.Code != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
