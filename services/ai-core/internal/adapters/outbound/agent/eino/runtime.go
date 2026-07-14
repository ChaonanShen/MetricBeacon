// Package eino implements the constrained, read-only node_exporter AgentRuntime.
//
// The Eino types live entirely at this outbound boundary. The application still
// depends only on the existing AgentRuntime and MetricCatalog/QueryEngine ports.
package eino

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/profile"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

const (
	maxToolSummaryBytes   = 16 << 10
	queryStepSeconds      = 300
	finalResponseProtocol = `

## Agent execution protocol

For every supported view, call query_prometheus with that view's exact canonical PromQL before replying. Do not describe a supported view as completed unless its query_prometheus call succeeded. You may call search_metrics and get_metric_labels only to identify supported metrics; never invent a tool result.

After all required tool calls, reply with exactly one JSON object and no Markdown or prose outside it:
{"status":"completed","views":["cpu"],"answer":"short user-visible conclusion"}

Use only the completed view keys cpu, memory, and load in views. If the request is unsupported, make no tool calls and reply exactly:
{"status":"unsupported","views":[],"answer":"short user-visible explanation"}`
)

type Limits struct {
	MaxIterations int
	MaxToolCalls  int
	Timeout       time.Duration
}

type Runtime struct {
	model   model.ToolCallingChatModel
	catalog tools.MetricCatalog
	queries tools.QueryEngine
	profile profile.Profile
	limits  Limits
}

var _ agent.Runtime = (*Runtime)(nil)

func New(chatModel model.ToolCallingChatModel, catalog tools.MetricCatalog, queries tools.QueryEngine, nodeProfile profile.Profile, limits Limits) (*Runtime, error) {
	if chatModel == nil || catalog == nil || queries == nil {
		return nil, fmt.Errorf("Eino runtime requires a model and metric ports")
	}
	if err := nodeProfile.Validate(); err != nil {
		return nil, err
	}
	if limits.MaxIterations <= 0 {
		limits.MaxIterations = 6
	}
	if limits.MaxToolCalls <= 0 {
		limits.MaxToolCalls = 12
	}
	return &Runtime{model: chatModel, catalog: catalog, queries: queries, profile: nodeProfile, limits: limits}, nil
}

func (r *Runtime) Run(ctx context.Context, identity requestcontext.Context, request dto.AgentRunRequest, sink agent.EventSink) (dto.AgentRunResult, error) {
	if r.limits.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.limits.Timeout)
		defer cancel()
	}
	if strings.TrimSpace(request.UserMessage) == "" {
		return dto.AgentRunResult{}, common.NewError(common.InvalidArgument, "user message is required", false)
	}
	if err := sink.Emit(ctx, dto.AgentEvent{Type: "assistant.message.started", Payload: map[string]any{}}); err != nil {
		return dto.AgentRunResult{}, err
	}

	run := &runState{runtime: r, identity: identity, request: request, sink: sink, proposals: make(map[string]dto.ChartProposal), sourceCallIDs: make(map[string]struct{})}
	chatAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "node_exporter_analysis",
		Description:   "Creates constrained node_exporter CPU, memory, and load analyses.",
		Instruction:   r.profile.Content + finalResponseProtocol,
		Model:         r.model,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: run.tools(), ExecuteSequentially: true}},
		MaxIterations: r.limits.MaxIterations,
	})
	if err != nil {
		return dto.AgentRunResult{}, common.NewError(common.InternalError, "cannot initialize the analysis runtime", false)
	}

	var final string
	iterator := chatAgent.Run(ctx, &adk.AgentInput{Messages: agentMessages(request)})
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return dto.AgentRunResult{}, agentFailure(event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil || event.Output.MessageOutput.Role != schema.Assistant {
			continue
		}
		message, messageErr := event.Output.MessageOutput.GetMessage()
		if messageErr != nil {
			return dto.AgentRunResult{}, common.NewError(common.DependencyUnavailable, "analysis model returned an unreadable response", true)
		}
		if message != nil && len(message.ToolCalls) == 0 && strings.TrimSpace(message.Content) != "" {
			final = message.Content
		}
	}

	result, err := run.finish(final)
	if err != nil {
		return dto.AgentRunResult{}, err
	}
	if err := sink.Emit(ctx, dto.AgentEvent{Type: "assistant.message.delta", Payload: result.AssistantText}); err != nil {
		return dto.AgentRunResult{}, err
	}
	return result, nil
}

func (r *Runtime) Resume(context.Context, requestcontext.Context, dto.AgentResumeRequest, agent.EventSink) (dto.AgentRunResult, error) {
	return dto.AgentRunResult{}, common.NewError(common.NotImplemented, "Eino agent resume is not implemented", false)
}

type runState struct {
	runtime       *Runtime
	identity      requestcontext.Context
	request       dto.AgentRunRequest
	sink          agent.EventSink
	mu            sync.Mutex
	toolCalls     int
	sourceCallIDs map[string]struct{}
	proposals     map[string]dto.ChartProposal
}

func (s *runState) tools() []tool.BaseTool {
	return []tool.BaseTool{
		strictTool("search_metrics", "Search the supported node_exporter metric registry.", searchToolSchema(), s.search),
		strictTool("get_metric_labels", "Read allowed label names for one supported metric.", labelsToolSchema(), s.labels),
		strictTool("query_prometheus", "Query exactly one registered node_exporter view using its canonical PromQL.", queryToolSchema(), s.query),
	}
}

func (s *runState) begin(ctx context.Context, toolName string, payload map[string]any) (string, error) {
	sourceCallID := compose.GetToolCallID(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if sourceCallID == "" {
		return "", common.NewError(common.SchemaValidationFailed, "model tool call did not include an id", false)
	}
	if _, exists := s.sourceCallIDs[sourceCallID]; exists {
		return "", common.NewError(common.SchemaValidationFailed, "model reused a tool call id", false)
	}
	if s.toolCalls >= s.runtime.limits.MaxToolCalls {
		return "", common.NewError(common.ExecutionInterrupted, "analysis exceeded the tool call limit", false)
	}
	s.toolCalls++
	s.sourceCallIDs[sourceCallID] = struct{}{}
	payload["toolName"] = toolName
	if err := s.sink.Emit(ctx, dto.AgentEvent{Type: "tool.started", SourceCallID: sourceCallID, Payload: payload}); err != nil {
		return "", err
	}
	return sourceCallID, nil
}

func (s *runState) complete(ctx context.Context, sourceCallID, toolName string, payload map[string]any) error {
	payload["toolName"] = toolName
	return s.sink.Emit(ctx, dto.AgentEvent{Type: "tool.completed", SourceCallID: sourceCallID, Payload: payload})
}

func (s *runState) search(ctx context.Context, raw string) (string, error) {
	var input struct {
		Query string `json:"query"`
	}
	if err := decodeStrict(raw, &input); err != nil || strings.TrimSpace(input.Query) == "" {
		return s.invalidTool(ctx, "grafana.search_metrics", "invalid search_metrics input")
	}
	callID, err := s.begin(ctx, "grafana.search_metrics", map[string]any{"query": input.Query})
	if err != nil {
		return "", err
	}
	result, err := s.runtime.catalog.SearchMetrics(ctx, s.identity, dto.SearchMetricsRequest{DatasourceUID: s.request.DatasourceUID, Query: input.Query, Limit: 10})
	if err != nil {
		return "", err
	}
	type candidate struct {
		MetricName string   `json:"metricName"`
		Type       string   `json:"type"`
		LabelNames []string `json:"labelNames"`
	}
	items := make([]candidate, 0, len(result.Candidates))
	for _, value := range result.Candidates {
		items = append(items, candidate{MetricName: value.MetricName, Type: value.Type, LabelNames: append([]string(nil), value.Labels...)})
	}
	if err := s.complete(ctx, callID, "grafana.search_metrics", map[string]any{"candidateCount": len(items)}); err != nil {
		return "", err
	}
	if err := s.sink.Emit(ctx, dto.AgentEvent{Type: "metric.candidates_created", Payload: map[string]any{"candidates": candidatesEvent(result.Candidates)}}); err != nil {
		return "", err
	}
	return boundedSummary(map[string]any{"success": true, "metrics": items})
}

func (s *runState) labels(ctx context.Context, raw string) (string, error) {
	var input struct {
		MetricName string `json:"metricName"`
	}
	if err := decodeStrict(raw, &input); err != nil || !registeredMetric(input.MetricName) {
		return s.invalidTool(ctx, "grafana.get_metric_labels", "metric is outside the node_exporter registry")
	}
	callID, err := s.begin(ctx, "grafana.get_metric_labels", map[string]any{"metricName": input.MetricName})
	if err != nil {
		return "", err
	}
	result, err := s.runtime.catalog.GetMetricLabels(ctx, s.identity, dto.GetMetricLabelsRequest{DatasourceUID: s.request.DatasourceUID, MetricName: input.MetricName})
	if err != nil {
		return "", err
	}
	counts := make(map[string]int, len(result.SampleValues))
	for label, values := range result.SampleValues {
		counts[label] = len(values)
	}
	if err := s.complete(ctx, callID, "grafana.get_metric_labels", map[string]any{"metricName": input.MetricName, "labelCount": len(result.LabelNames)}); err != nil {
		return "", err
	}
	return boundedSummary(map[string]any{"success": true, "metricName": input.MetricName, "labelNames": result.LabelNames, "sampleCounts": counts})
}

func (s *runState) query(ctx context.Context, raw string) (string, error) {
	var input struct {
		View       string `json:"view"`
		Expression string `json:"expression"`
	}
	if err := decodeStrict(raw, &input); err != nil {
		return s.invalidTool(ctx, "grafana.query_prometheus", "view and expression must match the registered node_exporter query")
	}
	view, known := profile.ViewForKey(input.View)
	if !known || input.Expression != view.CanonicalExpression {
		return s.invalidTool(ctx, "grafana.query_prometheus", "view and expression must match the registered node_exporter query")
	}
	callID, err := s.begin(ctx, "grafana.query_prometheus", map[string]any{"chartKey": view.Key})
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	_, repeated := s.proposals[view.Key]
	s.mu.Unlock()
	if repeated {
		return s.finishInvalid(ctx, callID, "grafana.query_prometheus", "each view may be queried once")
	}
	validation, err := s.runtime.queries.Validate(ctx, s.identity, dto.ValidateQueryRequest{DatasourceUID: s.request.DatasourceUID, Expression: view.CanonicalExpression})
	if err != nil {
		return "", err
	}
	if !validation.Valid || validation.CanonicalExpression != view.CanonicalExpression {
		return "", common.NewError(common.SchemaValidationFailed, "query validation rejected the registered expression", false)
	}
	execution, err := s.runtime.queries.Execute(ctx, s.identity, dto.ExecuteQueryRequest{DatasourceUID: s.request.DatasourceUID, Expression: validation.CanonicalExpression, TimeRange: s.request.TimeRange, StepSeconds: queryStepSeconds})
	if err != nil {
		return "", err
	}
	proposal := dto.ChartProposal{Key: view.Key, Title: view.Title, Visualization: "timeseries", Unit: view.Unit, Query: chart.QuerySpec{RefID: view.RefID, Expression: validation.CanonicalExpression, Legend: "{{instance}}", DatasourceUID: s.request.DatasourceUID, TimeRange: s.request.TimeRange}, Execution: execution}
	s.mu.Lock()
	s.proposals[view.Key] = proposal
	s.mu.Unlock()
	if err := s.complete(ctx, callID, "grafana.query_prometheus", map[string]any{"chartKey": view.Key, "seriesCount": len(execution.Series)}); err != nil {
		return "", err
	}
	return querySummary(view.Key, execution)
}

func (s *runState) invalidTool(ctx context.Context, toolName, reason string) (string, error) {
	callID, err := s.begin(ctx, toolName, map[string]any{})
	if err != nil {
		return "", err
	}
	return s.finishInvalid(ctx, callID, toolName, reason)
}

func (s *runState) finishInvalid(ctx context.Context, callID, toolName, reason string) (string, error) {
	if err := s.complete(ctx, callID, toolName, map[string]any{"success": false}); err != nil {
		return "", err
	}
	return `{"success":false,"error":"` + reason + `"}`, nil
}

func (s *runState) finish(raw string) (dto.AgentRunResult, error) {
	var response struct {
		Status string   `json:"status"`
		Views  []string `json:"views"`
		Answer string   `json:"answer"`
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := decodeStrict(trimJSONFence(raw), &response); err != nil {
		return s.finishTextFallback(raw)
	}
	if !validAnswer(response.Answer) {
		return dto.AgentRunResult{}, common.NewError(common.DependencyUnavailable, "analysis model returned an invalid final response", true)
	}
	if response.Status == "unsupported" {
		if len(response.Views) != 0 || s.toolCalls != 0 || len(s.proposals) != 0 {
			return dto.AgentRunResult{}, common.NewError(common.DependencyUnavailable, "analysis model returned an inconsistent unsupported response", true)
		}
		return dto.AgentRunResult{AssistantText: response.Answer}, nil
	}
	if response.Status != "completed" || len(response.Views) == 0 || !sameViews(response.Views, s.proposals) {
		return dto.AgentRunResult{}, common.NewError(common.DependencyUnavailable, "analysis model returned views inconsistent with tool results", true)
	}
	proposals := make([]dto.ChartProposal, 0, len(profile.Views()))
	for _, view := range profile.Views() {
		if proposal, ok := s.proposals[view.Key]; ok {
			proposals = append(proposals, proposal)
		}
	}
	return dto.AgentRunResult{AssistantText: response.Answer, Proposals: proposals}, nil
}

func (s *runState) finishTextFallback(raw string) (dto.AgentRunResult, error) {
	answer := strings.TrimSpace(raw)
	if !validAnswer(answer) || len(s.proposals) == 0 {
		return dto.AgentRunResult{}, common.NewError(common.DependencyUnavailable, "analysis model returned an invalid final response", true)
	}
	proposals := make([]dto.ChartProposal, 0, len(profile.Views()))
	for _, view := range profile.Views() {
		if proposal, ok := s.proposals[view.Key]; ok {
			proposals = append(proposals, proposal)
		}
	}
	return dto.AgentRunResult{AssistantText: answer, Proposals: proposals}, nil
}

func validAnswer(answer string) bool {
	return utf8.ValidString(answer) && utf8.RuneCountInString(answer) > 0 && utf8.RuneCountInString(answer) <= 4096
}

func trimJSONFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") || !strings.HasSuffix(trimmed, "```") {
		return trimmed
	}
	trimmed = strings.TrimSuffix(strings.TrimPrefix(trimmed, "```json"), "```")
	if trimmed == raw {
		trimmed = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(raw), "```"), "```")
	}
	return strings.TrimSpace(trimmed)
}

func agentMessages(request dto.AgentRunRequest) []adk.Message {
	messages := make([]adk.Message, 0, len(request.History)+2)
	for _, historical := range request.History {
		switch historical.Role {
		case "user":
			messages = append(messages, schema.UserMessage(historical.Content))
		case "assistant":
			messages = append(messages, schema.AssistantMessage(historical.Content, nil))
		}
	}
	messages = append(messages, schema.UserMessage("Current request:\n"+request.UserMessage+"\nLogical datasource: "+request.DatasourceUID+"\nTime range: "+request.TimeRange.From.UTC().Format("2006-01-02T15:04:05Z")+" to "+request.TimeRange.To.UTC().Format("2006-01-02T15:04:05Z")))
	return messages
}

func agentFailure(err error) *common.DomainError {
	var domainErr *common.DomainError
	if errors.As(err, &domainErr) {
		return domainErr
	}
	if errors.Is(err, adk.ErrExceedMaxIterations) {
		return common.NewError(common.ExecutionInterrupted, "analysis exceeded the model iteration limit", false)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return common.NewError(common.ExecutionInterrupted, "analysis exceeded the total runtime limit", false)
	}
	return common.NewError(common.DependencyUnavailable, "analysis model or tool dependency is unavailable", true)
}

func decodeStrict(raw string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}

func boundedSummary(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", common.NewError(common.InternalError, "cannot summarize tool output", false)
	}
	if len(encoded) > maxToolSummaryBytes {
		return `{"success":true,"summary":"tool result count only"}`, nil
	}
	return string(encoded), nil
}

func querySummary(view string, execution dto.QueryExecutionResult) (string, error) {
	type statistics struct {
		Alias       string  `json:"alias"`
		Latest      float64 `json:"latest"`
		Min         float64 `json:"min"`
		Max         float64 `json:"max"`
		Mean        float64 `json:"mean"`
		SampleCount int     `json:"sampleCount"`
	}
	items := make([]statistics, 0, min(len(execution.Series), 5))
	for index, series := range execution.Series {
		if index == 5 {
			break
		}
		values := make([]float64, 0, len(series.Points))
		for _, point := range series.Points {
			if !math.IsNaN(point.Value) && !math.IsInf(point.Value, 0) {
				values = append(values, point.Value)
			}
		}
		if len(values) == 0 {
			continue
		}
		minimum, maximum, total := values[0], values[0], 0.0
		for _, value := range values {
			minimum, maximum, total = math.Min(minimum, value), math.Max(maximum, value), total+value
		}
		items = append(items, statistics{Alias: fmt.Sprintf("series-%d", index+1), Latest: values[len(values)-1], Min: minimum, Max: maximum, Mean: total / float64(len(values)), SampleCount: len(values)})
	}
	return boundedSummary(map[string]any{"success": true, "view": view, "seriesCount": len(execution.Series), "series": items, "warnings": localWarnings(execution.Warnings)})
}

func localWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	return []string{"query returned local data warnings"}
}

func candidatesEvent(values []dto.MetricCandidate) []map[string]any {
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		result = append(result, map[string]any{"metricName": value.MetricName, "type": value.Type, "labels": value.Labels, "score": value.Score})
	}
	return result
}

func registeredMetric(name string) bool {
	return name == "node_cpu_seconds_total" || name == "node_memory_MemAvailable_bytes" || name == "node_memory_MemTotal_bytes" || name == "node_load1"
}

func sameViews(got []string, proposals map[string]dto.ChartProposal) bool {
	if len(got) != len(proposals) {
		return false
	}
	seen := make(map[string]struct{}, len(got))
	for _, view := range got {
		if _, exists := proposals[view]; !exists {
			return false
		}
		if _, duplicate := seen[view]; duplicate {
			return false
		}
		seen[view] = struct{}{}
	}
	return true
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

type strictInvokableTool struct {
	info *schema.ToolInfo
	run  func(context.Context, string) (string, error)
}

func strictTool(name, description string, params *schema.ParamsOneOf, run func(context.Context, string) (string, error)) tool.InvokableTool {
	return strictInvokableTool{info: &schema.ToolInfo{Name: name, Desc: description, ParamsOneOf: params}, run: run}
}

func (t strictInvokableTool) Info(context.Context) (*schema.ToolInfo, error) { return t.info, nil }
func (t strictInvokableTool) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	return t.run(ctx, arguments)
}

func searchToolSchema() *schema.ParamsOneOf {
	return strictObject(map[string]*jsonschema.Schema{"query": {Type: "string", Description: "Metric search text."}}, []string{"query"})
}

func labelsToolSchema() *schema.ParamsOneOf {
	return strictObject(map[string]*jsonschema.Schema{"metricName": {Type: "string", Enum: []any{"node_cpu_seconds_total", "node_memory_MemAvailable_bytes", "node_memory_MemTotal_bytes", "node_load1"}}}, []string{"metricName"})
}

func queryToolSchema() *schema.ParamsOneOf {
	return strictObject(map[string]*jsonschema.Schema{"view": {Type: "string", Enum: []any{"cpu", "memory", "load"}}, "expression": {Type: "string"}}, []string{"view", "expression"})
}

func strictObject(properties map[string]*jsonschema.Schema, required []string) *schema.ParamsOneOf {
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := orderedmap.New[string, *jsonschema.Schema]()
	for _, key := range keys {
		ordered.Set(key, properties[key])
	}
	return schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{Type: "object", Required: required, AdditionalProperties: jsonschema.FalseSchema, Properties: ordered})
}
