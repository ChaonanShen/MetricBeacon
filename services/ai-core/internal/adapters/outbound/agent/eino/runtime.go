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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/localresult"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/profile"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

const (
	maxToolSummaryBytes   = 16 << 10
	finalResponseProtocol = `

## Agent execution protocol

For every supported view, call query_prometheus with only its view key before replying. The application injects the effective time range, step, CPU rate window, datasource, and canonical PromQL. Do not describe a supported view as completed unless its query_prometheus call succeeded. You may call search_metrics and get_metric_labels only to identify supported metrics; never invent a tool result.

After all required tool calls, reply with exactly one JSON object and no Markdown or prose outside it:
{"status":"completed","views":["cpu"]}

Use only the completed view keys cpu, memory, and load in views. If the request is unsupported, make no tool calls and reply exactly:
{"status":"unsupported","views":[]}`
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
		strictTool("query_prometheus", "Query exactly one registered node_exporter view. All query parameters are injected locally.", queryToolSchema(), s.query),
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
		View string `json:"view"`
	}
	if err := decodeStrict(raw, &input); err != nil {
		return s.invalidTool(ctx, "grafana.query_prometheus", "view must be a registered node_exporter query")
	}
	view, known := profile.ViewForKey(input.View)
	if !known {
		return s.invalidTool(ctx, "grafana.query_prometheus", "view must be a registered node_exporter query")
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
	cpuWindow := cpuWindowForView(view.Key, s.request.QueryPlan.CPURateWindowSeconds)
	validation, err := s.runtime.queries.Validate(ctx, s.identity, dto.ValidateQueryRequest{DatasourceUID: s.request.DatasourceUID, View: view.Key, CPURateWindowSeconds: cpuWindow})
	if err != nil {
		return "", err
	}
	if !validation.Valid || validation.CanonicalExpression == "" {
		return "", common.NewError(common.SchemaValidationFailed, "query validation rejected the registered expression", false)
	}
	execution, err := s.runtime.queries.Execute(ctx, s.identity, dto.ExecuteQueryRequest{DatasourceUID: s.request.DatasourceUID, View: view.Key, CPURateWindowSeconds: cpuWindow, TimeRange: s.request.TimeRange, StepSeconds: s.request.QueryPlan.StepSeconds})
	if err != nil {
		return "", err
	}
	proposal := dto.ChartProposal{Key: view.Key, Title: view.Title, Visualization: "timeseries", Unit: view.Unit, Query: chart.QuerySpec{RefID: view.RefID, Expression: validation.CanonicalExpression, Legend: "{{instance}}", DatasourceUID: s.request.DatasourceUID, TimeRange: s.request.TimeRange, StepSeconds: s.request.QueryPlan.StepSeconds}, Execution: execution}
	s.mu.Lock()
	s.proposals[view.Key] = proposal
	s.mu.Unlock()
	if err := s.complete(ctx, callID, "grafana.query_prometheus", map[string]any{"chartKey": view.Key, "seriesCount": len(execution.Series)}); err != nil {
		return "", err
	}
	return querySummary(s.request, proposal)
}

func cpuWindowForView(view string, value int) *int {
	if view != "cpu" {
		return nil
	}
	return &value
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
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := decodeStrict(trimJSONFence(raw), &response); err != nil {
		return dto.AgentRunResult{}, common.NewError(common.DependencyUnavailable, "analysis model returned an invalid final response", true)
	}
	if response.Status == "unsupported" {
		if len(response.Views) != 0 || s.toolCalls != 0 || len(s.proposals) != 0 {
			return dto.AgentRunResult{}, common.NewError(common.DependencyUnavailable, "analysis model returned an inconsistent unsupported response", true)
		}
		return dto.AgentRunResult{AssistantText: "当前仅支持 node_exporter 的 CPU、内存和系统负载视图。"}, nil
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
	return dto.AgentRunResult{AssistantText: localresult.Format(s.request, proposals), Proposals: proposals}, nil
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

func querySummary(request dto.AgentRunRequest, proposal dto.ChartProposal) (string, error) {
	statistics := localresult.Summarize(proposal)
	var cpuWindow any
	if proposal.Key == "cpu" {
		cpuWindow = request.QueryPlan.CPURateWindowSeconds
	}
	data := map[string]any{"seriesCount": statistics.SeriesCount, "sampleCount": statistics.SampleCount}
	if statistics.HasData {
		data["first"], data["latest"] = statistics.First, statistics.Latest
		data["min"], data["max"], data["mean"], data["delta"] = statistics.Min, statistics.Max, statistics.Mean, statistics.Delta
		data["actualRange"] = map[string]any{"from": statistics.ActualFrom, "to": statistics.ActualTo}
	}
	return boundedSummary(map[string]any{
		"success": true,
		"view":    proposal.Key,
		"effectiveQuery": map[string]any{
			"rangeSeconds":         int(request.TimeRange.To.Sub(request.TimeRange.From).Seconds()),
			"stepSeconds":          request.QueryPlan.StepSeconds,
			"cpuRateWindowSeconds": cpuWindow,
		},
		"data":     data,
		"warnings": localWarnings(proposal.Execution.Warnings),
	})
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
	return strictObject(map[string]*jsonschema.Schema{"view": {Type: "string", Enum: []any{"cpu", "memory", "load"}}}, []string{"view"})
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
