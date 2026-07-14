// Package mock implements the deterministic AgentRuntime replacement point.
package mock

import (
	"context"
	"strings"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

const (
	CPUQuery    = `100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`
	MemoryQuery = `100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes`
	LoadQuery   = `node_load1`
)

type Runtime struct {
	catalog tools.MetricCatalog
	queries tools.QueryEngine
}

var _ agent.Runtime = (*Runtime)(nil)

func New(catalog tools.MetricCatalog, queries tools.QueryEngine) *Runtime {
	return &Runtime{catalog: catalog, queries: queries}
}

func (r *Runtime) Run(ctx context.Context, identity requestcontext.Context, request dto.AgentRunRequest, sink agent.EventSink) (dto.AgentRunResult, error) {
	if strings.TrimSpace(request.UserMessage) == "" {
		return dto.AgentRunResult{}, common.NewError(common.InvalidArgument, "user message is required", false)
	}
	if err := emit(sink, ctx, "assistant.message.started", map[string]any{}); err != nil {
		return dto.AgentRunResult{}, err
	}
	if err := sink.Emit(ctx, dto.AgentEvent{Type: "assistant.message.delta", Payload: "正在生成固定的 node_exporter 分析视图…"}); err != nil {
		return dto.AgentRunResult{}, err
	}
	if err := emitTool(sink, ctx, "tool.started", "mock-01-search", map[string]any{"toolName": "grafana.search_metrics"}); err != nil {
		return dto.AgentRunResult{}, err
	}
	metrics, err := r.catalog.SearchMetrics(ctx, identity, dto.SearchMetricsRequest{DatasourceUID: request.DatasourceUID, Query: "node exporter cpu memory load", Limit: 10})
	if err != nil {
		return dto.AgentRunResult{}, err
	}
	if err := emitTool(sink, ctx, "tool.completed", "mock-01-search", map[string]any{"toolName": "grafana.search_metrics", "candidateCount": len(metrics.Candidates)}); err != nil {
		return dto.AgentRunResult{}, err
	}
	if !containsMetrics(metrics, "node_cpu_seconds_total", "node_memory_MemAvailable_bytes", "node_load1") {
		return dto.AgentRunResult{}, common.NewError(common.SchemaValidationFailed, "metric search did not return the required node_exporter metrics", false)
	}
	if err := emit(sink, ctx, "metric.candidates_created", map[string]any{"candidates": metricCandidatesWire(metrics.Candidates)}); err != nil {
		return dto.AgentRunResult{}, err
	}
	for index, metric := range []string{"node_cpu_seconds_total", "node_memory_MemAvailable_bytes", "node_load1"} {
		callID := []string{"mock-02-labels-cpu", "mock-03-labels-memory", "mock-04-labels-load"}[index]
		if err := emitTool(sink, ctx, "tool.started", callID, map[string]any{"toolName": "grafana.get_metric_labels", "metricName": metric}); err != nil {
			return dto.AgentRunResult{}, err
		}
		labels, err := r.catalog.GetMetricLabels(ctx, identity, dto.GetMetricLabelsRequest{DatasourceUID: request.DatasourceUID, MetricName: metric})
		if err != nil {
			return dto.AgentRunResult{}, err
		}
		if err := emitTool(sink, ctx, "tool.completed", callID, map[string]any{"toolName": "grafana.get_metric_labels", "metricName": metric, "labelCount": len(labels.LabelNames)}); err != nil {
			return dto.AgentRunResult{}, err
		}
		if !contains(labels.LabelNames, "instance") {
			return dto.AgentRunResult{}, common.NewError(common.SchemaValidationFailed, "metric labels did not include instance", false)
		}
	}
	proposals := make([]dto.ChartProposal, 0, 3)
	for index, spec := range []struct{ key, title, unit, expression string }{{"cpu", "CPU 使用率", "percent", CPUQuery}, {"memory", "内存可用率", "percent", MemoryQuery}, {"load", "系统负载", "short", LoadQuery}} {
		callID := []string{"mock-05-query-cpu", "mock-06-query-memory", "mock-07-query-load"}[index]
		if err := emitTool(sink, ctx, "tool.started", callID, map[string]any{"toolName": "grafana.query_prometheus", "chartKey": spec.key}); err != nil {
			return dto.AgentRunResult{}, err
		}
		validation, err := r.queries.Validate(ctx, identity, dto.ValidateQueryRequest{DatasourceUID: request.DatasourceUID, Expression: spec.expression})
		if err != nil {
			return dto.AgentRunResult{}, err
		}
		if !validation.Valid || validation.CanonicalExpression == "" {
			return dto.AgentRunResult{}, common.NewError(common.SchemaValidationFailed, "Prometheus query validation did not return a canonical expression", false)
		}
		execution, err := r.queries.Execute(ctx, identity, dto.ExecuteQueryRequest{DatasourceUID: request.DatasourceUID, Expression: validation.CanonicalExpression, TimeRange: request.TimeRange, StepSeconds: request.QueryPlan.StepSeconds})
		if err != nil {
			return dto.AgentRunResult{}, err
		}
		if err := emitTool(sink, ctx, "tool.completed", callID, map[string]any{"toolName": "grafana.query_prometheus", "chartKey": spec.key, "seriesCount": len(execution.Series)}); err != nil {
			return dto.AgentRunResult{}, err
		}
		proposals = append(proposals, dto.ChartProposal{Key: spec.key, Title: spec.title, Visualization: "timeseries", Unit: spec.unit, Query: chart.QuerySpec{RefID: strings.ToUpper(spec.key[:1]), Expression: validation.CanonicalExpression, Legend: "{{instance}}", DatasourceUID: request.DatasourceUID, TimeRange: request.TimeRange, StepSeconds: request.QueryPlan.StepSeconds}, Execution: execution})
	}
	return dto.AgentRunResult{AssistantText: "已生成 node_exporter 的 CPU、内存和系统负载视图。", Proposals: proposals}, nil
}

func emit(sink agent.EventSink, ctx context.Context, eventType string, payload any) error {
	return sink.Emit(ctx, dto.AgentEvent{Type: eventType, Payload: payload})
}
func emitTool(sink agent.EventSink, ctx context.Context, eventType, sourceCallID string, payload any) error {
	return sink.Emit(ctx, dto.AgentEvent{Type: eventType, SourceCallID: sourceCallID, Payload: payload})
}
func (r *Runtime) Resume(context.Context, requestcontext.Context, dto.AgentResumeRequest, agent.EventSink) (dto.AgentRunResult, error) {
	return dto.AgentRunResult{}, common.NewError(common.NotImplemented, "mock agent resume is not implemented", false)
}
func containsMetrics(result dto.SearchMetricsResult, names ...string) bool {
	for _, name := range names {
		found := false
		for _, candidate := range result.Candidates {
			if candidate.MetricName == name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func metricCandidatesWire(values []dto.MetricCandidate) []map[string]any {
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		sources := make([]map[string]any, 0, len(value.Sources))
		for _, source := range value.Sources {
			sources = append(sources, map[string]any{"type": source.Type, "reference": source.Reference})
		}
		result = append(result, map[string]any{"metricName": value.MetricName, "type": value.Type, "description": value.Description, "labels": value.Labels, "score": value.Score, "sources": sources})
	}
	return result
}
