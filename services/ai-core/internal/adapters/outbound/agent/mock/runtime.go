// Package mock contains deterministic outbound Agent adapters.
package mock

import (
	"context"
	"strings"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/localresult"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/profile"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

type Runtime struct{ queries tools.QueryEngine }

var _ agent.Runtime = (*Runtime)(nil)

// New keeps the catalog argument for assembly compatibility; deterministic
// execution no longer performs fixed search or label calls.
func New(_ tools.MetricCatalog, queries tools.QueryEngine) *Runtime {
	return &Runtime{queries: queries}
}

func (r *Runtime) Run(ctx context.Context, identity requestcontext.Context, request dto.AgentRunRequest, sink agent.EventSink) (dto.AgentRunResult, error) {
	if strings.TrimSpace(request.UserMessage) == "" {
		return dto.AgentRunResult{}, common.NewError(common.InvalidArgument, "user message is required", false)
	}
	if err := emit(sink, ctx, "assistant.message.started", map[string]any{}); err != nil {
		return dto.AgentRunResult{}, err
	}
	if len(request.QueryPlan.Views) == 0 {
		text := "当前仅支持 node_exporter 的 CPU、内存和系统负载视图。"
		if err := sink.Emit(ctx, dto.AgentEvent{Type: "assistant.message.delta", Payload: text}); err != nil {
			return dto.AgentRunResult{}, err
		}
		return dto.AgentRunResult{AssistantText: text}, nil
	}
	if err := sink.Emit(ctx, dto.AgentEvent{Type: "assistant.message.delta", Payload: "正在执行已冻结的 node_exporter 分析计划…"}); err != nil {
		return dto.AgentRunResult{}, err
	}
	proposals := make([]dto.ChartProposal, 0, len(request.QueryPlan.Views))
	for _, key := range request.QueryPlan.Views {
		view, known := profile.ViewForKey(key)
		if !known {
			return dto.AgentRunResult{}, common.NewError(common.SchemaValidationFailed, "persisted query view is outside the registry", false)
		}
		callID := "query-" + key
		if err := emitTool(sink, ctx, "tool.started", callID, map[string]any{"toolName": "grafana.query_prometheus", "chartKey": key}); err != nil {
			return dto.AgentRunResult{}, err
		}
		cpuWindow := cpuWindowForView(key, request.QueryPlan.CPURateWindowSeconds)
		validation, err := r.queries.Validate(ctx, identity, dto.ValidateQueryRequest{DatasourceUID: request.DatasourceUID, View: key, CPURateWindowSeconds: cpuWindow})
		if err != nil {
			return dto.AgentRunResult{}, err
		}
		if !validation.Valid || validation.CanonicalExpression == "" {
			return dto.AgentRunResult{}, common.NewError(common.SchemaValidationFailed, "Prometheus query validation did not return a canonical expression", false)
		}
		execution, err := r.queries.Execute(ctx, identity, dto.ExecuteQueryRequest{DatasourceUID: request.DatasourceUID, View: key, CPURateWindowSeconds: cpuWindow, TimeRange: request.TimeRange, StepSeconds: request.QueryPlan.StepSeconds})
		if err != nil {
			return dto.AgentRunResult{}, err
		}
		if err := emitTool(sink, ctx, "tool.completed", callID, map[string]any{"toolName": "grafana.query_prometheus", "chartKey": key, "seriesCount": len(execution.Series)}); err != nil {
			return dto.AgentRunResult{}, err
		}
		proposals = append(proposals, dto.ChartProposal{Key: key, Title: view.Title, Visualization: "timeseries", Unit: view.Unit, Query: chart.QuerySpec{RefID: view.RefID, Expression: validation.CanonicalExpression, Legend: "{{instance}}", DatasourceUID: request.DatasourceUID, TimeRange: request.TimeRange, StepSeconds: request.QueryPlan.StepSeconds}, Execution: execution})
	}
	return dto.AgentRunResult{AssistantText: localresult.Format(request, proposals), Proposals: proposals}, nil
}

func cpuWindowForView(view string, value int) *int {
	if view != "cpu" {
		return nil
	}
	return &value
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
