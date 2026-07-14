package mock

import (
	"context"
	"errors"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
)

func TestRunBuildsFixedPlanThroughPorts(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC)
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	runtime := New(catalogStub{}, queryStub{})
	result, err := runtime.Run(context.Background(), requestcontext.Context{TenantID: "org:1", OrgID: "1", UserID: "user:1"}, dto.AgentRunRequest{TaskID: "task_1", SessionID: "session_1", UserMessage: "anything", DatasourceUID: "prometheus-main", TimeRange: timeRange, QueryPlan: task.LegacyQueryPlan()}, sinkStub{})
	if err != nil {
		t.Fatal(err)
	}
	if result.AssistantText != "已生成 node_exporter 的 CPU、内存和系统负载视图。" || len(result.Proposals) != 3 || result.Proposals[0].Key != "cpu" || result.Proposals[2].Key != "load" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunRejectsMissingRequiredMetric(t *testing.T) {
	now := time.Now().UTC()
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-time.Minute), now)
	_, err := New(catalogStub{missing: true}, queryStub{}).Run(context.Background(), requestcontext.Context{}, dto.AgentRunRequest{UserMessage: "anything", DatasourceUID: "prometheus-main", TimeRange: timeRange, QueryPlan: task.LegacyQueryPlan()}, sinkStub{})
	var domainErr *common.DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != common.SchemaValidationFailed {
		t.Fatalf("expected schema error, got %v", err)
	}
}

type catalogStub struct{ missing bool }

func (s catalogStub) SearchMetrics(context.Context, requestcontext.Context, dto.SearchMetricsRequest) (dto.SearchMetricsResult, error) {
	candidates := []dto.MetricCandidate{{MetricName: "node_cpu_seconds_total"}, {MetricName: "node_memory_MemAvailable_bytes"}, {MetricName: "node_load1"}}
	if s.missing {
		candidates = candidates[:2]
	}
	return dto.SearchMetricsResult{Candidates: candidates}, nil
}
func (catalogStub) GetMetricLabels(context.Context, requestcontext.Context, dto.GetMetricLabelsRequest) (dto.MetricLabelsResult, error) {
	return dto.MetricLabelsResult{LabelNames: []string{"instance"}}, nil
}

type queryStub struct{}

func (queryStub) Validate(_ context.Context, _ requestcontext.Context, request dto.ValidateQueryRequest) (dto.QueryValidationResult, error) {
	return dto.QueryValidationResult{Valid: true, CanonicalExpression: canonicalForView(request.View)}, nil
}

func canonicalForView(view string) string {
	if view == "cpu" {
		return "cpu_registered_query"
	}
	return "node_" + view
}
func (queryStub) Execute(context.Context, requestcontext.Context, dto.ExecuteQueryRequest) (dto.QueryExecutionResult, error) {
	return dto.QueryExecutionResult{Status: "success", Series: []chart.Series{{Name: "node-a"}}, Validation: dto.QueryValidationResult{Valid: true}}, nil
}

type sinkStub struct{}

func (sinkStub) Emit(context.Context, dto.AgentEvent) error { return nil }
