package bootstrap_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	approvalevidence "mini-torchbearing.local/packages/approval-evidence-go"
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

	search := call(t, context, client, "grafana.search_metrics", map[string]any{"datasourceUid": "prometheus-main", "query": "node exporter", "limit": 10})
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

	labels := call(t, context, client, "grafana.get_metric_labels", map[string]any{"datasourceUid": "prometheus-main", "metricName": "node_cpu_seconds_total"})
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
	query := call(t, context, client, "grafana.query_prometheus", map[string]any{"datasourceUid": "prometheus-main", "view": "cpu", "cpuRateWindowSeconds": 60, "start": start.Format(time.RFC3339), "end": start.Add(30 * time.Minute).Format(time.RFC3339), "stepSeconds": 300, "mode": "execute"})
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
	unauthorized := call(t, context, unauthorizedClient, "grafana.search_metrics", map[string]any{"datasourceUid": "prometheus-main", "query": "node exporter", "limit": 10})
	if !unauthorized.IsError {
		t.Fatal("request without datasources:query unexpectedly succeeded")
	}
}

func TestRemediationProfileRequiresEvidenceUsesTypedWriteAndAudits(t *testing.T) {
	now := time.Now().UTC()
	auditPath := filepath.Join(t.TempDir(), "execution-audit.jsonl")
	evidenceKey := "0123456789abcdef0123456789abcdef"
	runtimeValue, err := bootstrap.Wire(bootstrap.Config{
		FixtureDir: repositoryPath(t, "data/mock-scenarios/node_exporter_overview"), SchemaDir: repositoryPath(t, "contracts/tools/grafana"),
		IncidentEnabled: true, RemediationEnabled: true, IncidentToolSchemaDir: repositoryPath(t, "contracts/tools/incident"), AssetDir: repositoryPath(t, "data/operational-assets"), AssetSchemaDir: repositoryPath(t, "contracts/schemas/assets"),
		OrderDriver: "mock", OrderMockScenario: "worker-stopped", CheckpointKey: evidenceKey, ApprovalEvidenceKey: evidenceKey, ExecutionAuditPath: auditPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(runtimeValue.Handler)
	t.Cleanup(server.Close)
	ctx := context.Background()
	clientValue := newClient(t, server.URL+"/mcp", remediationHeaders())
	initialize(t, ctx, clientValue)
	tools, err := clientValue.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil || len(tools.Tools) != 17 {
		t.Fatalf("tools=%#v err=%v", tools.Tools, err)
	}
	foundWrite := false
	for _, tool := range tools.Tools {
		if tool.Name == "order_service.restore_worker_concurrency" {
			foundWrite = true
			if tool.Annotations.ReadOnlyHint == nil || *tool.Annotations.ReadOnlyHint || tool.Annotations.IdempotentHint == nil || !*tool.Annotations.IdempotentHint || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
				t.Fatalf("write annotations=%#v", tool.Annotations)
			}
		}
	}
	if !foundWrite {
		t.Fatal("typed write tool is missing")
	}

	digest := "sha256:" + strings.Repeat("a", 64)
	codec, _ := approvalevidence.New([]byte(evidenceKey))
	token, err := codec.Sign(approvalevidence.Claims{Version: approvalevidence.Version, TenantID: "org:1", OrgID: "1", TaskID: "task-1", ApprovalID: "approval-1", IntentDigest: digest, CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: "order-demo", InstanceEpoch: "mock-epoch-1", ExpectedVersion: 2, OperationID: "operation-1", IssuedAt: now, ExpiresAt: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	arguments := map[string]any{"operationId": "operation-1", "instanceEpoch": "mock-epoch-1", "expectedVersion": 2, "expectedConcurrency": 0, "newConcurrency": 2, "intentDigest": digest, "approvalId": "approval-1", "approvalEvidence": token}
	tampered := mapsClone(arguments)
	tampered["approvalEvidence"] = token + "tampered"
	if result := call(t, ctx, clientValue, "order_service.restore_worker_concurrency", tampered); !result.IsError {
		t.Fatal("tampered ApprovalEvidence was accepted")
	}
	workerBefore := call(t, ctx, clientValue, "order_service.get_worker_state", map[string]any{})
	var before struct{ ConfiguredConcurrency int }
	decodeStructured(t, workerBefore, &before)
	if before.ConfiguredConcurrency != 0 {
		t.Fatalf("invalid evidence changed worker: %#v", before)
	}

	executed := call(t, ctx, clientValue, "order_service.restore_worker_concurrency", arguments)
	if executed.IsError {
		t.Fatalf("execute failed: %#v", executed)
	}
	var receipt struct {
		OperationId, IntentDigest, ApprovalId string
		BeforeVersion, AfterVersion           int
	}
	decodeStructured(t, executed, &receipt)
	if receipt.OperationId != "operation-1" || receipt.IntentDigest != digest || receipt.BeforeVersion != 2 || receipt.AfterVersion != 3 {
		t.Fatalf("receipt=%#v", receipt)
	}
	retry := call(t, ctx, clientValue, "order_service.restore_worker_concurrency", arguments)
	if retry.IsError {
		t.Fatalf("idempotent retry failed: %#v", retry)
	}
	metrics := call(t, ctx, clientValue, "order_service.get_recovery_metrics", map[string]any{})
	var metricsOutput struct {
		WindowSeconds                             int
		AcceptedDelta, CompletedDelta, QueueDepth float64
	}
	decodeStructured(t, metrics, &metricsOutput)
	if metrics.IsError || metricsOutput.WindowSeconds != 30 || metricsOutput.CompletedDelta < metricsOutput.AcceptedDelta || metricsOutput.QueueDepth != 0 {
		t.Fatalf("metrics=%#v result=%#v", metricsOutput, metrics)
	}
	probe := call(t, ctx, clientValue, "order_service.run_business_probe", map[string]any{"probeId": "probe-1"})
	var probeOutput struct {
		ProbeId, Result string
		DurationMs      int
	}
	decodeStructured(t, probe, &probeOutput)
	if probe.IsError || probeOutput.Result != "completed" || probeOutput.DurationMs > 5000 {
		t.Fatalf("probe=%#v result=%#v", probeOutput, probe)
	}
	contents, err := os.ReadFile(auditPath)
	if err != nil || strings.Count(string(contents), "\n") != 4 || strings.Contains(string(contents), "approvalEvidence") {
		t.Fatalf("audit=%q err=%v", contents, err)
	}
}

func mapsClone(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func TestHTTPDriverReadinessAndInvalidConfiguration(t *testing.T) {
	prometheus := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/-/ready" {
			t.Fatalf("unexpected readiness request: %s", request.URL.Path)
		}
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(prometheus.Close)
	runtime, err := bootstrap.Wire(bootstrap.Config{PrometheusDriver: "http", PrometheusURL: prometheus.URL, PrometheusTimeout: time.Second, PrometheusDatasourceUID: "prometheus-main", SchemaDir: repositoryPath(t, "contracts/tools/grafana")})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(runtime.Handler)
	t.Cleanup(server.Close)
	response, err := http.Get(server.URL + "/healthz")
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("healthz should remain process-only: %v", err)
	}
	response.Body.Close()
	response, err = http.Get(server.URL + "/readyz")
	if err != nil || response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("readyz did not report the unavailable Prometheus dependency: %v", err)
	}
	response.Body.Close()
	_, err = bootstrap.Wire(bootstrap.Config{PrometheusDriver: "invalid", FixtureDir: repositoryPath(t, "data/mock-scenarios/node_exporter_overview"), SchemaDir: repositoryPath(t, "contracts/tools/grafana")})
	if err == nil {
		t.Fatal("invalid Prometheus driver was accepted")
	}
}

func TestIncidentProfileRegistersOnlyBoundedToolsAndRunsStoppedWorkerDiagnosis(t *testing.T) {
	runtime, err := bootstrap.Wire(bootstrap.Config{
		FixtureDir: repositoryPath(t, "data/mock-scenarios/node_exporter_overview"), SchemaDir: repositoryPath(t, "contracts/tools/grafana"),
		IncidentEnabled: true, IncidentToolSchemaDir: repositoryPath(t, "contracts/tools/incident"), AssetDir: repositoryPath(t, "data/operational-assets"), AssetSchemaDir: repositoryPath(t, "contracts/schemas/assets"),
		OrderDriver: "mock", OrderMockScenario: "worker-stopped", CheckpointKey: "0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(runtime.Handler)
	t.Cleanup(server.Close)
	context := context.Background()
	client := newClient(t, server.URL+"/mcp", incidentHeaders(true))
	initialize(t, context, client)
	tools, err := client.ListTools(context, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 14 {
		t.Fatalf("incident profile has unexpected tools: %#v", tools.Tools)
	}
	for _, tool := range tools.Tools {
		lower := strings.ToLower(tool.Name)
		if strings.Contains(lower, "fault") || strings.Contains(lower, "shell") || strings.Contains(lower, "execute") {
			t.Fatalf("unsafe tool was exposed: %s", tool.Name)
		}
		if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Fatalf("tool is not closed-world read-only: %#v", tool)
		}
	}

	knowledge := call(t, context, client, "knowledge.get_document", map[string]any{"id": "order-service", "version": "1"})
	if knowledge.IsError {
		t.Fatalf("Knowledge failed: %#v", knowledge)
	}
	var knowledgeOutput struct{ Digest, Content string }
	decodeStructured(t, knowledge, &knowledgeOutput)
	if len(knowledgeOutput.Digest) != 64 || !strings.Contains(knowledgeOutput.Content, "Backlog is a symptom") {
		t.Fatalf("unexpected Knowledge: %#v", knowledgeOutput)
	}

	resolved := call(t, context, client, "playbook.resolve_alert", map[string]any{"sourceId": "demo-grafana", "alertName": "OrderQueueBacklog", "labels": map[string]string{"service_ref": "order-demo", "severity": "warning"}})
	if resolved.IsError {
		t.Fatalf("resolve failed: %#v", resolved)
	}
	var resolution struct{ PlaybookId, PlaybookVersion, PlaybookDigest, ServiceRef string }
	decodeStructured(t, resolved, &resolution)
	started := call(t, context, client, "playbook.start_run", map[string]any{"playbookId": resolution.PlaybookId, "version": resolution.PlaybookVersion, "digest": resolution.PlaybookDigest, "serviceRef": resolution.ServiceRef})
	if started.IsError {
		t.Fatalf("start failed: %#v", started)
	}
	var startOutput struct {
		Status        string
		Checkpoint    string
		AssetRefs     []any
		CapabilityIds []string
	}
	decodeStructured(t, started, &startOutput)
	if startOutput.Status != "needs_agent" || len(startOutput.AssetRefs) != 3 || len(startOutput.CapabilityIds) != 6 {
		t.Fatalf("unexpected start: %#v", startOutput)
	}
	worker := call(t, context, client, "order_service.get_worker_state", map[string]any{})
	if worker.IsError {
		t.Fatalf("worker read failed: %#v", worker)
	}
	var workerOutput struct{ ConfiguredConcurrency, EffectiveConcurrency, ActiveWorkers, Version int }
	decodeStructured(t, worker, &workerOutput)
	if workerOutput.ConfiguredConcurrency != 0 || workerOutput.EffectiveConcurrency != 0 || workerOutput.ActiveWorkers != 0 || workerOutput.Version != 2 {
		t.Fatalf("unexpected worker state: %#v", workerOutput)
	}
	resumed := call(t, context, client, "playbook.resume_run", map[string]any{"checkpoint": startOutput.Checkpoint, "diagnosis": map[string]any{"primaryHypothesis": "workers are stopped", "evidenceRefs": []string{"order_service.get_worker_state", "order_service.get_worker_policy"}, "alternatives": []string{"slow processing"}, "confidence": 0.95, "candidateAction": "restore_worker_concurrency"}})
	if resumed.IsError {
		t.Fatalf("resume failed: %#v", resumed)
	}
	var resumeOutput struct {
		Status      string
		IntentDraft *struct {
			CapabilityId      string
			InstanceEpoch     string
			ExpectedVersion   int
			BeforeConcurrency int
			AfterConcurrency  int
			PolicyDigest      string
			PlaybookDigest    string
		}
	}
	decodeStructured(t, resumed, &resumeOutput)
	if resumeOutput.Status != "needs_approval" || resumeOutput.IntentDraft == nil || resumeOutput.IntentDraft.CapabilityId != "order_service.restore_worker_concurrency" || resumeOutput.IntentDraft.InstanceEpoch == "" || resumeOutput.IntentDraft.ExpectedVersion != 2 || resumeOutput.IntentDraft.BeforeConcurrency != 0 || resumeOutput.IntentDraft.AfterConcurrency != 2 || len(resumeOutput.IntentDraft.PolicyDigest) != 64 || len(resumeOutput.IntentDraft.PlaybookDigest) != 64 {
		t.Fatalf("unexpected version-bound Intent: %#v", resumeOutput)
	}
	for _, test := range []struct {
		name string
		args map[string]any
	}{
		{"order_service.get_worker_state", map[string]any{"command": "anything"}},
		{"playbook.resolve_alert", map[string]any{"sourceId": "demo-grafana", "alertName": "Unknown", "labels": map[string]string{"service_ref": "order-demo"}}},
		{"order_service.get_operation", map[string]any{"operationId": "missing"}},
		{"playbook.resume_run", map[string]any{"checkpoint": startOutput.Checkpoint + "tampered", "diagnosis": map[string]any{"primaryHypothesis": "stopped", "evidenceRefs": []string{"order_service.get_worker_state"}, "alternatives": []string{}, "confidence": 1, "candidateAction": "restore_worker_concurrency"}}},
	} {
		if result := call(t, context, client, test.name, test.args); !result.IsError {
			t.Fatalf("fail-closed case unexpectedly succeeded for %s: %#v", test.name, result)
		}
	}

	unauthorized := newClient(t, server.URL+"/mcp", incidentHeaders(false))
	initialize(t, context, unauthorized)
	denied := call(t, context, unauthorized, "order_service.get_worker_state", map[string]any{})
	if !denied.IsError {
		t.Fatal("incident tool succeeded without incidents:diagnose")
	}
}

func TestIncidentProfileSimilarSymptomsDoNotProduceIntent(t *testing.T) {
	for _, scenario := range []string{"healthy", "slow-processing", "dependency-errors"} {
		t.Run(scenario, func(t *testing.T) {
			runtime, err := bootstrap.Wire(bootstrap.Config{
				FixtureDir: repositoryPath(t, "data/mock-scenarios/node_exporter_overview"), SchemaDir: repositoryPath(t, "contracts/tools/grafana"),
				IncidentEnabled: true, IncidentToolSchemaDir: repositoryPath(t, "contracts/tools/incident"), AssetDir: repositoryPath(t, "data/operational-assets"), AssetSchemaDir: repositoryPath(t, "contracts/schemas/assets"),
				OrderDriver: "mock", OrderMockScenario: scenario, CheckpointKey: "0123456789abcdef0123456789abcdef",
			})
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(runtime.Handler)
			t.Cleanup(server.Close)
			ctx := context.Background()
			client := newClient(t, server.URL+"/mcp", incidentHeaders(true))
			initialize(t, ctx, client)
			resolved := call(t, ctx, client, "playbook.resolve_alert", map[string]any{"sourceId": "demo-grafana", "alertName": "OrderQueueBacklog", "labels": map[string]string{"service_ref": "order-demo"}})
			var resolution struct{ PlaybookId, PlaybookVersion, PlaybookDigest, ServiceRef string }
			decodeStructured(t, resolved, &resolution)
			started := call(t, ctx, client, "playbook.start_run", map[string]any{"playbookId": resolution.PlaybookId, "version": resolution.PlaybookVersion, "digest": resolution.PlaybookDigest, "serviceRef": resolution.ServiceRef})
			var start struct{ Checkpoint string }
			decodeStructured(t, started, &start)
			resumed := call(t, ctx, client, "playbook.resume_run", map[string]any{"checkpoint": start.Checkpoint, "diagnosis": map[string]any{"primaryHypothesis": "requesting restore despite alternative evidence", "evidenceRefs": []string{"order_service.get_worker_state", "order_service.get_worker_policy"}, "alternatives": []string{"dependency errors"}, "confidence": 0.99, "candidateAction": "restore_worker_concurrency"}})
			if resumed.IsError {
				t.Fatalf("no-action resume failed: %#v", resumed)
			}
			var output struct {
				Status      string
				IntentDraft any `json:"intentDraft"`
			}
			decodeStructured(t, resumed, &output)
			if output.Status != "completed" || output.IntentDraft != nil {
				t.Fatalf("scenario %s produced an unsafe Intent: %#v", scenario, output)
			}
		})
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

func incidentHeaders(withPermission bool) map[string]string {
	result := headers(true)
	if withPermission {
		result["X-MTB-Permissions"] = "datasources:query,incidents:diagnose"
	}
	return result
}

func remediationHeaders() map[string]string {
	result := headers(true)
	result["X-MTB-Permissions"] = "incidents:remediate"
	return result
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
