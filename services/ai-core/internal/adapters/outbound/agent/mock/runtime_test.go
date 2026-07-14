package mock

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
)

func TestRunExecutesOnlyPersistedViewsInOrder(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC)
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	queries := &queryStub{}
	runtime := New(catalogStub{}, queries)
	plan, _ := task.NewQueryPlan([]string{"load", "cpu"}, 300, 300)
	result, err := runtime.Run(context.Background(), requestcontext.Context{TenantID: "org:1", OrgID: "1", UserID: "user:1"}, dto.AgentRunRequest{TaskID: "task_1", SessionID: "session_1", UserMessage: "anything", DatasourceUID: "prometheus-main", TimeRange: timeRange, QueryPlan: plan}, sinkStub{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.AssistantText, "30分钟") || !strings.Contains(result.AssistantText, "step=300s") || !strings.Contains(result.AssistantText, "CPU rate window=300s") || len(result.Proposals) != 2 || result.Proposals[0].Key != "load" || result.Proposals[1].Key != "cpu" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if strings.Join(queries.views, ",") != "load,load,cpu,cpu" {
		t.Fatalf("query calls = %#v", queries.views)
	}
}

func TestRunRejectsUnknownPersistedView(t *testing.T) {
	now := time.Now().UTC()
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-time.Minute), now)
	_, err := New(catalogStub{}, &queryStub{}).Run(context.Background(), requestcontext.Context{}, dto.AgentRunRequest{UserMessage: "anything", DatasourceUID: "prometheus-main", TimeRange: timeRange, QueryPlan: task.QueryPlan{Views: []string{"disk"}, StepSeconds: 5, CPURateWindowSeconds: 30}}, sinkStub{})
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

func TestRunCompletesUnsupportedWithoutQueryAccess(t *testing.T) {
	now := time.Now().UTC()
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-time.Minute), now)
	queries := &queryStub{}
	result, err := New(catalogStub{}, queries).Run(context.Background(), requestcontext.Context{}, dto.AgentRunRequest{UserMessage: "disk", DatasourceUID: "prometheus-main", TimeRange: timeRange, QueryPlan: task.QueryPlan{Views: []string{}, StepSeconds: 5, CPURateWindowSeconds: 30}}, sinkStub{})
	if err != nil || len(result.Proposals) != 0 || len(queries.views) != 0 || !strings.Contains(result.AssistantText, "仅支持") {
		t.Fatalf("result = %#v, calls = %#v, err = %v", result, queries.views, err)
	}
}

type queryStub struct{ views []string }

func (q *queryStub) Validate(_ context.Context, _ requestcontext.Context, request dto.ValidateQueryRequest) (dto.QueryValidationResult, error) {
	q.views = append(q.views, request.View)
	return dto.QueryValidationResult{Valid: true, CanonicalExpression: canonicalForView(request.View)}, nil
}

func canonicalForView(view string) string {
	if view == "cpu" {
		return "cpu_registered_query"
	}
	return "node_" + view
}
func (q *queryStub) Execute(_ context.Context, _ requestcontext.Context, request dto.ExecuteQueryRequest) (dto.QueryExecutionResult, error) {
	q.views = append(q.views, request.View)
	return dto.QueryExecutionResult{Status: "success", Series: []chart.Series{{Name: "node-a", Points: []chart.Point{{Timestamp: request.TimeRange.From, Value: 10}, {Timestamp: request.TimeRange.To, Value: 20}}}}, Validation: dto.QueryValidationResult{Valid: true}}, nil
}

type sinkStub struct{}

func (sinkStub) Emit(context.Context, dto.AgentEvent) error { return nil }
