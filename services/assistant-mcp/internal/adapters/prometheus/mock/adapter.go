package mock

import (
	"context"
	"strings"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/adapters/prometheus/registry"
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
	if request.DatasourceUID != registry.DatasourceUID || strings.TrimSpace(request.Query) == "" || request.Limit < 1 || request.Limit > 100 {
		return prometheus.SearchMetricsResult{}, runtime.NewError(runtime.SchemaValidationFailed, "search metrics request is invalid", false)
	}
	result := prometheus.SearchMetricsResult{Candidates: append([]prometheus.MetricCandidate(nil), a.fixture.Search.Candidates...)}
	if len(result.Candidates) > request.Limit {
		result.Candidates = result.Candidates[:request.Limit]
	}
	return result, nil
}

func (a *Adapter) GetMetricLabels(_ context.Context, _ requestcontext.Context, request prometheus.GetMetricLabelsRequest) (prometheus.MetricLabelsResult, error) {
	if request.DatasourceUID != registry.DatasourceUID || !registry.IsRegisteredMetric(request.MetricName) {
		return prometheus.MetricLabelsResult{}, runtime.NewError(runtime.SchemaValidationFailed, "metric labels request is invalid", false)
	}
	result, ok := a.fixture.Labels[request.MetricName]
	if !ok {
		return prometheus.MetricLabelsResult{}, runtime.NewError(runtime.SchemaValidationFailed, "metric is not available in the mock scenario", false)
	}
	return cloneLabels(result), nil
}

func (a *Adapter) Query(_ context.Context, _ requestcontext.Context, request prometheus.QueryRequest) (prometheus.QueryResult, error) {
	if request.DatasourceUID != registry.DatasourceUID || !request.Start.Before(request.End) || request.End.Sub(request.Start) > 6*time.Hour || !validStep(request.StepSeconds) || int(request.End.Sub(request.Start).Seconds())/request.StepSeconds+1 > 1000 || (request.Mode != prometheus.ModeValidate && request.Mode != prometheus.ModeExecute) {
		return prometheus.QueryResult{}, runtime.NewError(runtime.SchemaValidationFailed, "Prometheus query request is invalid", false)
	}
	definition, err := registry.Resolve(request.View, request.CPURateWindowSeconds)
	if err != nil {
		return prometheus.QueryResult{}, runtime.NewError(runtime.SchemaValidationFailed, "node_exporter view parameters are outside the registry", false)
	}
	result, ok := a.fixture.Queries[definition.View]
	if !ok {
		return prometheus.QueryResult{}, runtime.NewError(runtime.DependencyUnavailable, "mock scenario does not provide the registered query", true)
	}
	result.Validation = prometheus.Validation{Valid: true, Errors: []string{}, Warnings: append([]string{}, result.Validation.Warnings...), MetricNames: append([]string{}, definition.MetricNames...), LabelNames: append([]string{}, definition.LabelNames...), CanonicalExpression: definition.CanonicalExpression}
	if request.Mode == prometheus.ModeValidate {
		result.Series = []prometheus.Series{}
		return result, nil
	}
	return resampleToRange(result, request.Start, request.End, request.StepSeconds), nil
}

func validStep(value int) bool {
	switch value {
	case 5, 10, 15, 30, 60, 120, 300:
		return true
	default:
		return false
	}
}

func cloneLabels(value prometheus.MetricLabelsResult) prometheus.MetricLabelsResult {
	result := prometheus.MetricLabelsResult{MetricName: value.MetricName, LabelNames: append([]string(nil), value.LabelNames...), SampleValues: make(map[string][]string, len(value.SampleValues))}
	for key, values := range value.SampleValues {
		result.SampleValues[key] = append([]string(nil), values...)
	}
	return result
}
