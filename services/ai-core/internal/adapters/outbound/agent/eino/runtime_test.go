package eino_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/eino"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/profile"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
)

func TestRuntimeKeepsRawSeriesOutOfModelInput(t *testing.T) {
	model := &scriptedModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{ID: "query-cpu", Type: "function", Function: schema.FunctionCall{Name: "query_prometheus", Arguments: `{"view":"cpu"}`}}}),
		schema.AssistantMessage(`{"status":"completed","views":["cpu"]}`, nil),
	}}
	queries := &fakeQueries{execution: dto.QueryExecutionResult{Status: "success", Series: []chart.Series{{Name: "private-label", Labels: map[string]string{"instance": "http://private.example/secret"}, Points: []chart.Point{{Timestamp: time.Unix(1, 0), Value: 31}, {Timestamp: time.Unix(2, 0), Value: 37}}}}, Warnings: []string{"sensitive upstream warning"}}}
	runtime := newRuntime(t, model, &fakeCatalog{}, queries)
	sink := &recordingSink{}
	result, err := runtime.Run(context.Background(), identity(), request(), sink)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Proposals) != 1 || result.Proposals[0].Key != "cpu" || result.Proposals[0].Query.RefID != "A" {
		t.Fatalf("unexpected chart proposals: %#v", result.Proposals)
	}
	if !strings.Contains(result.AssistantText, "step=300s") || !strings.Contains(result.AssistantText, "CPU rate window=300s") || !strings.Contains(result.AssistantText, "首值 31.00%") || strings.Contains(result.AssistantText, "CPU 视图已生成") {
		t.Fatalf("final answer was not produced from local query facts: %s", result.AssistantText)
	}
	if queries.execute.StepSeconds != 300 || queries.execute.View != "cpu" || queries.execute.CPURateWindowSeconds == nil || *queries.execute.CPURateWindowSeconds != 300 {
		t.Fatalf("query was not constrained to the canonical execution: %#v", queries.execute)
	}
	if !sink.hasToolPair("query-cpu", "grafana.query_prometheus") {
		t.Fatalf("durable tool events do not preserve the source call id: %#v", sink.events)
	}
	inputs, err := json.Marshal(model.inputs)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private-label", "private.example", "sensitive upstream warning"} {
		if strings.Contains(string(inputs), forbidden) {
			t.Fatalf("model input leaked %q: %s", forbidden, inputs)
		}
	}
	for _, required := range []string{"call query_prometheus", `\"status\":\"completed\"`, `\"status\":\"unsupported\"`, `\"stepSeconds\":300`, `\"cpuRateWindowSeconds\":300`} {
		if !strings.Contains(string(inputs), required) {
			t.Fatalf("model input did not include the constrained execution protocol %q: %s", required, inputs)
		}
	}
	if !strings.Contains(string(inputs), `\"sampleCount\":2`) || strings.Contains(string(inputs), `\"expression\"`) {
		t.Fatalf("model did not receive the bounded local summary: %s", inputs)
	}
}

func TestRuntimeRejectsFinalViewsWithoutSuccessfulTools(t *testing.T) {
	runtime := newRuntime(t, &scriptedModel{responses: []*schema.Message{schema.AssistantMessage(`{"status":"completed","views":["cpu"]}`, nil)}}, &fakeCatalog{}, &fakeQueries{})
	_, err := runtime.Run(context.Background(), identity(), request(), &recordingSink{})
	assertCode(t, err, common.DependencyUnavailable)
}

func TestRuntimeRejectsPlainTextFinalResponseAfterSuccessfulQuery(t *testing.T) {
	model := &scriptedModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{ID: "query-cpu", Type: "function", Function: schema.FunctionCall{Name: "query_prometheus", Arguments: `{"view":"cpu"}`}}}),
		schema.AssistantMessage("CPU 视图已生成。", nil),
	}}
	runtime := newRuntime(t, model, &fakeCatalog{}, &fakeQueries{execution: dto.QueryExecutionResult{
		Status: "success",
		Series: []chart.Series{{
			Name:   "cpu",
			Points: []chart.Point{{Timestamp: time.Unix(1, 0), Value: 31}},
		}},
	}})
	_, err := runtime.Run(context.Background(), identity(), request(), &recordingSink{})
	assertCode(t, err, common.DependencyUnavailable)
}

func TestRuntimeStrictlyRejectsUnknownToolFieldsBeforeCatalogAccess(t *testing.T) {
	catalog := &fakeCatalog{}
	model := &scriptedModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{ID: "bad-search", Type: "function", Function: schema.FunctionCall{Name: "search_metrics", Arguments: `{"query":"cpu","endpoint":"http://private.example"}`}}}),
		schema.AssistantMessage(`{"status":"unsupported","views":[]}`, nil),
	}}
	sink := &recordingSink{}
	runtime := newRuntime(t, model, catalog, &fakeQueries{})
	_, err := runtime.Run(context.Background(), identity(), request(), sink)
	assertCode(t, err, common.DependencyUnavailable)
	if catalog.searches != 0 {
		t.Fatalf("invalid tool input reached MetricCatalog: %d", catalog.searches)
	}
	if !sink.hasToolPair("bad-search", "grafana.search_metrics") {
		t.Fatalf("invalid call was not durably correlated: %#v", sink.events)
	}
}

func TestRuntimeRejectsModelSuppliedPromQLBeforeQueryAccess(t *testing.T) {
	queries := &fakeQueries{}
	model := &scriptedModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{ID: "bad-query", Type: "function", Function: schema.FunctionCall{Name: "query_prometheus", Arguments: `{"view":"cpu","expression":"up"}`}}}),
		schema.AssistantMessage(`{"status":"unsupported","views":[]}`, nil),
	}}
	_, err := newRuntime(t, model, &fakeCatalog{}, queries).Run(context.Background(), identity(), request(), &recordingSink{})
	assertCode(t, err, common.DependencyUnavailable)
	if queries.validations != 0 || queries.executions != 0 {
		t.Fatalf("model-supplied PromQL reached QueryEngine: %#v", queries)
	}
}

func newRuntime(t *testing.T, chatModel model.ToolCallingChatModel, catalog *fakeCatalog, queries *fakeQueries) *eino.Runtime {
	t.Helper()
	nodeProfile, err := profile.Load(repositoryPath(t, "data/agent-knowledge/node_exporter.md"))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := eino.New(chatModel, catalog, queries, nodeProfile, eino.Limits{MaxIterations: 6, MaxToolCalls: 12})
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

type scriptedModel struct {
	mu        sync.Mutex
	responses []*schema.Message
	inputs    [][]*schema.Message
}

func (m *scriptedModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
func (m *scriptedModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var copied []*schema.Message
	if err := json.Unmarshal(encoded, &copied); err != nil {
		return nil, err
	}
	m.inputs = append(m.inputs, copied)
	if len(m.responses) == 0 {
		return nil, errors.New("unexpected model call")
	}
	next := m.responses[0]
	m.responses = m.responses[1:]
	return next, nil
}
func (m *scriptedModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("streaming is disabled")
}

type fakeCatalog struct{ searches int }

func (c *fakeCatalog) SearchMetrics(context.Context, requestcontext.Context, dto.SearchMetricsRequest) (dto.SearchMetricsResult, error) {
	c.searches++
	return dto.SearchMetricsResult{}, nil
}
func (*fakeCatalog) GetMetricLabels(context.Context, requestcontext.Context, dto.GetMetricLabelsRequest) (dto.MetricLabelsResult, error) {
	return dto.MetricLabelsResult{}, nil
}

type fakeQueries struct {
	execution   dto.QueryExecutionResult
	execute     dto.ExecuteQueryRequest
	validations int
	executions  int
}

func (q *fakeQueries) Validate(_ context.Context, _ requestcontext.Context, input dto.ValidateQueryRequest) (dto.QueryValidationResult, error) {
	q.validations++
	canonical := "node_" + input.View
	if input.View == "cpu" {
		canonical = `100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`
	}
	return dto.QueryValidationResult{Valid: true, CanonicalExpression: canonical}, nil
}
func (q *fakeQueries) Execute(_ context.Context, _ requestcontext.Context, input dto.ExecuteQueryRequest) (dto.QueryExecutionResult, error) {
	q.execute = input
	q.executions++
	return q.execution, nil
}

type recordingSink struct {
	mu     sync.Mutex
	events []dto.AgentEvent
}

func (s *recordingSink) Emit(_ context.Context, event dto.AgentEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}
func (s *recordingSink) hasToolPair(callID, toolName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	started, completed := false, false
	for _, event := range s.events {
		payload, _ := event.Payload.(map[string]any)
		if event.SourceCallID == callID && payload["toolName"] == toolName {
			started = started || event.Type == "tool.started"
			completed = completed || event.Type == "tool.completed"
		}
	}
	return started && completed
}

func identity() requestcontext.Context {
	return requestcontext.Context{TenantID: "tenant-a", OrgID: "1", UserID: "user-a", Permissions: []string{"datasources:query"}}
}

func request() dto.AgentRunRequest {
	return dto.AgentRunRequest{TaskID: "task-a", SessionID: "session-a", UserMessage: "只看 CPU", DatasourceUID: "prometheus-main", TimeRange: common.AbsoluteTimeRange{From: time.Unix(0, 0).UTC(), To: time.Unix(300, 0).UTC()}, QueryPlan: task.LegacyQueryPlan()}
}

func assertCode(t *testing.T, err error, want common.ErrorCode) {
	t.Helper()
	var got *common.DomainError
	if !errors.As(err, &got) || got.Code != want {
		t.Fatalf("error = %v, want code %q", err, want)
	}
}

func repositoryPath(t *testing.T, relative string) string {
	t.Helper()
	directory, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(directory, relative)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatalf("repository path %q was not found", relative)
		}
		directory = parent
	}
}
