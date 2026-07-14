package mock

import (
	"context"
	"strings"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/prometheus"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

// Adapter is the only assistant-mcp code allowed to load the deterministic
// node_exporter fixture. It implements the same Prometheus port as the future
// real adapter.
type Adapter struct{ fixture fixture }

var _ prometheus.Port = (*Adapter)(nil)

func New(directory string) (*Adapter, error) {
	loaded, err := loadFixture(directory)
	if err != nil {
		return nil, err
	}
	return &Adapter{fixture: loaded}, nil
}

func (a *Adapter) SearchMetrics(_ context.Context, _ requestcontext.Context, request prometheus.SearchMetricsRequest) (prometheus.SearchMetricsResult, error) {
	if request.DatasourceUID != "prometheus-main" || strings.TrimSpace(request.Query) == "" || request.Limit < 1 || request.Limit > 100 {
		return prometheus.SearchMetricsResult{}, runtime.NewError(runtime.SchemaValidationFailed, "search metrics request is invalid", false)
	}
	result := prometheus.SearchMetricsResult{Candidates: append([]prometheus.MetricCandidate(nil), a.fixture.Search.Candidates...)}
	if len(result.Candidates) > request.Limit {
		result.Candidates = result.Candidates[:request.Limit]
	}
	return result, nil
}

func (a *Adapter) GetMetricLabels(_ context.Context, _ requestcontext.Context, request prometheus.GetMetricLabelsRequest) (prometheus.MetricLabelsResult, error) {
	if request.DatasourceUID != "prometheus-main" {
		return prometheus.MetricLabelsResult{}, runtime.NewError(runtime.SchemaValidationFailed, "metric labels request is invalid", false)
	}
	result, ok := a.fixture.Labels[request.MetricName]
	if !ok {
		return prometheus.MetricLabelsResult{}, runtime.NewError(runtime.SchemaValidationFailed, "metric is not available in the mock scenario", false)
	}
	return cloneLabels(result), nil
}

func (a *Adapter) Query(_ context.Context, _ requestcontext.Context, request prometheus.QueryRequest) (prometheus.QueryResult, error) {
	if request.DatasourceUID != "prometheus-main" || !request.Start.Before(request.End) || request.StepSeconds < 1 || request.StepSeconds > 3600 || (request.Mode != prometheus.ModeValidate && request.Mode != prometheus.ModeExecute) {
		return prometheus.QueryResult{}, runtime.NewError(runtime.SchemaValidationFailed, "Prometheus query request is invalid", false)
	}
	result, ok := a.fixture.Queries[request.Expression]
	if !ok {
		return prometheus.QueryResult{}, runtime.NewError(runtime.SchemaValidationFailed, "PromQL expression is not available in the mock scenario", false)
	}
	if request.Mode == prometheus.ModeValidate {
		result.Series = []prometheus.Series{}
		return result, nil
	}
	return shiftToRange(result, request.Start), nil
}

func cloneLabels(value prometheus.MetricLabelsResult) prometheus.MetricLabelsResult {
	result := prometheus.MetricLabelsResult{MetricName: value.MetricName, LabelNames: append([]string(nil), value.LabelNames...), SampleValues: make(map[string][]string, len(value.SampleValues))}
	for key, values := range value.SampleValues {
		result.SampleValues[key] = append([]string(nil), values...)
	}
	return result
}
