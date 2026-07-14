package http_test

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	prometheushttp "mini-torchbearing.local/services/assistant-mcp/internal/adapters/prometheus/http"
	"mini-torchbearing.local/services/assistant-mcp/internal/adapters/prometheus/registry"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/prometheus"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

func TestAdapterContract(t *testing.T) {
	var requests []string
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		switch request.URL.Path {
		case "/api/v1/metadata":
			metric := request.URL.Query().Get("metric")
			writeJSON(t, writer, map[string]any{"status": "success", "data": map[string]any{metric: []map[string]string{{"type": "gauge", "help": "node exporter " + metric}}}})
		case "/api/v1/series":
			if err := request.ParseForm(); err != nil || request.Form.Get("match[]") != "node_cpu_seconds_total" {
				t.Fatalf("unexpected series request: %v %#v", err, request.Form)
			}
			writeJSON(t, writer, map[string]any{"status": "success", "data": []map[string]string{{"__name__": "node_cpu_seconds_total", "instance": "node-b:9100", "mode": "idle"}, {"__name__": "node_cpu_seconds_total", "instance": "node-a:9100", "mode": "idle"}}})
		case "/api/v1/query_range":
			if err := request.ParseForm(); err != nil || request.Form.Get("query") != registry.Definitions()[0].CanonicalExpression {
				t.Fatalf("query did not use canonical expression: %v %#v", err, request.Form)
			}
			writeJSON(t, writer, map[string]any{"status": "success", "data": map[string]any{"resultType": "matrix", "result": []map[string]any{{"metric": map[string]string{"__name__": "node_cpu", "instance": "node-a:9100"}, "values": [][]any{{1720872000, "22"}, {1720872300, "NaN"}}}}}})
		case "/-/ready":
			writer.WriteHeader(stdhttp.StatusOK)
		default:
			writer.WriteHeader(stdhttp.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	adapter, err := prometheushttp.New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	identity := requestcontext.Context{TenantID: "org:1"}
	search, err := adapter.SearchMetrics(context.Background(), identity, prometheus.SearchMetricsRequest{DatasourceUID: registry.DatasourceUID, Query: "node exporter", Limit: 10})
	if err != nil || len(search.Candidates) != 4 || search.Candidates[0].Sources[0].Type != "prometheus_metadata" {
		t.Fatalf("unexpected search result: %#v, %v", search, err)
	}
	labels, err := adapter.GetMetricLabels(context.Background(), identity, prometheus.GetMetricLabelsRequest{DatasourceUID: registry.DatasourceUID, MetricName: "node_cpu_seconds_total"})
	if err != nil || strings.Join(labels.LabelNames, ",") != "instance,mode" || strings.Join(labels.SampleValues["instance"], ",") != "node-a:9100,node-b:9100" {
		t.Fatalf("unexpected labels result: %#v, %v", labels, err)
	}
	start := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	query, err := adapter.Query(context.Background(), identity, prometheus.QueryRequest{DatasourceUID: registry.DatasourceUID, Expression: " ( " + registry.Definitions()[0].CanonicalExpression + " ) ", Start: start, End: start.Add(30 * time.Minute), StepSeconds: 300, Mode: prometheus.ModeExecute})
	if err != nil || len(query.Series) != 1 || len(query.Series[0].Points) != 1 || len(query.Warnings) != 1 {
		t.Fatalf("unexpected query result: %#v, %v", query, err)
	}
	if err := adapter.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Join(requests, ",") != "GET /api/v1/metadata,GET /api/v1/metadata,GET /api/v1/metadata,GET /api/v1/metadata,POST /api/v1/series,POST /api/v1/query_range,GET /-/ready" {
		t.Fatalf("unexpected HTTP calls: %v", requests)
	}
}

func TestValidateDoesNotContactPrometheusAndRejectsLimits(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) { t.Fatal("validation contacted Prometheus") }))
	t.Cleanup(server.Close)
	adapter, err := prometheushttp.New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC()
	result, err := adapter.Query(context.Background(), requestcontext.Context{}, prometheus.QueryRequest{DatasourceUID: registry.DatasourceUID, Expression: registry.Definitions()[2].CanonicalExpression, Start: start, End: start.Add(time.Minute), StepSeconds: 60, Mode: prometheus.ModeValidate})
	if err != nil || !result.Validation.Valid || result.Validation.CanonicalExpression != registry.Definitions()[2].CanonicalExpression {
		t.Fatalf("unexpected validation: %#v, %v", result, err)
	}
	_, err = adapter.Query(context.Background(), requestcontext.Context{}, prometheus.QueryRequest{DatasourceUID: registry.DatasourceUID, Expression: registry.Definitions()[2].CanonicalExpression, Start: start, End: start.Add(6*time.Hour + time.Second), StepSeconds: 60, Mode: prometheus.ModeExecute})
	requireCode(t, err, runtime.SchemaValidationFailed)
}

func TestSearchUsesDeterministicLocalMatching(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
		metric := request.URL.Query().Get("metric")
		writeJSON(t, writer, map[string]any{"status": "success", "data": map[string]any{metric: []map[string]string{{"type": "gauge", "help": metric}}}})
	}))
	t.Cleanup(server.Close)
	adapter, err := prometheushttp.New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.SearchMetrics(context.Background(), requestcontext.Context{}, prometheus.SearchMetricsRequest{DatasourceUID: registry.DatasourceUID, Query: "cpu", Limit: 10})
	if err != nil || len(result.Candidates) != 1 || result.Candidates[0].MetricName != "node_cpu_seconds_total" {
		t.Fatalf("unexpected local search result: %#v, %v", result, err)
	}
}

func TestAdapterMapsUnauthorizedTimeoutAndMalformedResponses(t *testing.T) {
	cases := []struct {
		name    string
		handler stdhttp.HandlerFunc
		timeout time.Duration
		want    runtime.ErrorCode
	}{
		{"unauthorized", func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
			writer.WriteHeader(stdhttp.StatusUnauthorized)
		}, time.Second, runtime.AdapterNotConfigured},
		{"timeout", func(_ stdhttp.ResponseWriter, _ *stdhttp.Request) { time.Sleep(100 * time.Millisecond) }, 10 * time.Millisecond, runtime.ToolTimeout},
		{"rate_limited", func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
			writer.WriteHeader(stdhttp.StatusTooManyRequests)
		}, time.Second, runtime.DependencyUnavailable},
		{"malformed", func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) { _, _ = writer.Write([]byte("not-json")) }, time.Second, runtime.DependencyUnavailable},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			t.Cleanup(server.Close)
			adapter, err := prometheushttp.New(server.URL, test.timeout)
			if err != nil {
				t.Fatal(err)
			}
			_, err = adapter.SearchMetrics(context.Background(), requestcontext.Context{}, prometheus.SearchMetricsRequest{DatasourceUID: registry.DatasourceUID, Query: "node", Limit: 1})
			requireCode(t, err, test.want)
		})
	}
}

func TestAdapterRejectsResponseLimits(t *testing.T) {
	start := time.Now().UTC()
	cases := []struct {
		name    string
		handler stdhttp.HandlerFunc
		query   bool
		want    runtime.ErrorCode
	}{
		{"oversized_metadata", func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
			_, _ = writer.Write([]byte(strings.Repeat("x", 2<<20+1)))
		}, false, runtime.SchemaValidationFailed},
		{"too_many_series", func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
			result := make([]map[string]any, 21)
			for index := range result {
				result[index] = map[string]any{"metric": map[string]string{"__name__": "node_load1", "instance": "node"}, "values": [][]any{{float64(start.Unix()), "1"}}}
			}
			writeJSON(t, writer, map[string]any{"status": "success", "data": map[string]any{"resultType": "matrix", "result": result}})
		}, true, runtime.SchemaValidationFailed},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			t.Cleanup(server.Close)
			adapter, err := prometheushttp.New(server.URL, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if test.query {
				_, err = adapter.Query(context.Background(), requestcontext.Context{}, prometheus.QueryRequest{DatasourceUID: registry.DatasourceUID, Expression: registry.Definitions()[2].CanonicalExpression, Start: start, End: start.Add(time.Minute), StepSeconds: 60, Mode: prometheus.ModeExecute})
			} else {
				_, err = adapter.SearchMetrics(context.Background(), requestcontext.Context{}, prometheus.SearchMetricsRequest{DatasourceUID: registry.DatasourceUID, Query: "node", Limit: 1})
			}
			requireCode(t, err, test.want)
		})
	}
}

func TestAdapterRejectsRedirects(t *testing.T) {
	target := httptest.NewServer(stdhttp.HandlerFunc(func(_ stdhttp.ResponseWriter, _ *stdhttp.Request) { t.Fatal("redirect was followed") }))
	t.Cleanup(target.Close)
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
		writer.Header().Set("Location", target.URL)
		writer.WriteHeader(stdhttp.StatusFound)
	}))
	t.Cleanup(server.Close)
	adapter, err := prometheushttp.New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.SearchMetrics(context.Background(), requestcontext.Context{}, prometheus.SearchMetricsRequest{DatasourceUID: registry.DatasourceUID, Query: "node", Limit: 1})
	requireCode(t, err, runtime.DependencyUnavailable)
}

func TestAdapterConstructionRejectsInvalidConfiguration(t *testing.T) {
	for _, endpoint := range []string{"", "prometheus:9090", "ftp://prometheus:9090"} {
		if _, err := prometheushttp.New(endpoint, time.Second); err == nil {
			t.Fatalf("endpoint %q was accepted", endpoint)
		}
	}
	parsed, _ := url.Parse("http://prometheus:9090")
	if _, err := prometheushttp.NewWithHTTPClient(parsed, 0, stdhttp.DefaultClient); err == nil {
		t.Fatal("zero timeout was accepted")
	}
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
