// Package grafana implements the read-only grafana.* MCP namespace.
package grafana

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	generated "mini-torchbearing.local/packages/generated-contracts/go"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/prometheus"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

type Service struct{ prometheus prometheus.Port }

func NewService(port prometheus.Port) *Service { return &Service{prometheus: port} }

func (s *Service) SearchMetrics(ctx context.Context, identity requestcontext.Context, input generated.SearchMetricsInputSchema) (generated.SearchMetricsOutputSchema, error) {
	if err := authorize(identity); err != nil {
		return generated.SearchMetricsOutputSchema{}, err
	}
	datasourceUID := string(input.DatasourceUid)
	if datasourceUID != "prometheus-main" || strings.TrimSpace(input.Query) == "" || input.Limit < 1 || input.Limit > 100 {
		return generated.SearchMetricsOutputSchema{}, runtime.NewError(runtime.SchemaValidationFailed, "search metrics input does not match the tool schema", false)
	}
	result, err := s.prometheus.SearchMetrics(ctx, identity, prometheus.SearchMetricsRequest{DatasourceUID: datasourceUID, Query: input.Query, Limit: input.Limit})
	if err != nil {
		return generated.SearchMetricsOutputSchema{}, err
	}
	var output generated.SearchMetricsOutputSchema
	if err := mapValue(result, &output); err != nil {
		return generated.SearchMetricsOutputSchema{}, err
	}
	if err := validateSearchOutput(output); err != nil {
		return generated.SearchMetricsOutputSchema{}, err
	}
	return output, nil
}

func (s *Service) GetMetricLabels(ctx context.Context, identity requestcontext.Context, input generated.GetMetricLabelsInputSchema) (generated.GetMetricLabelsOutputSchema, error) {
	if err := authorize(identity); err != nil {
		return generated.GetMetricLabelsOutputSchema{}, err
	}
	datasourceUID := string(input.DatasourceUid)
	if datasourceUID != "prometheus-main" || !input.MetricName.Valid() {
		return generated.GetMetricLabelsOutputSchema{}, runtime.NewError(runtime.SchemaValidationFailed, "metric labels input does not match the tool schema", false)
	}
	result, err := s.prometheus.GetMetricLabels(ctx, identity, prometheus.GetMetricLabelsRequest{DatasourceUID: datasourceUID, MetricName: string(input.MetricName)})
	if err != nil {
		return generated.GetMetricLabelsOutputSchema{}, err
	}
	var output generated.GetMetricLabelsOutputSchema
	if err := mapValue(result, &output); err != nil {
		return generated.GetMetricLabelsOutputSchema{}, err
	}
	if err := validateLabelsOutput(output); err != nil {
		return generated.GetMetricLabelsOutputSchema{}, err
	}
	return output, nil
}

func (s *Service) QueryPrometheus(ctx context.Context, identity requestcontext.Context, input generated.QueryPrometheusInputSchema) (generated.QueryPrometheusOutputSchema, error) {
	if err := authorize(identity); err != nil {
		return generated.QueryPrometheusOutputSchema{}, err
	}
	datasourceUID := string(input.DatasourceUid)
	view := string(input.View)
	var cpuWindow *int
	if input.CpuRateWindowSeconds != nil {
		value := int(*input.CpuRateWindowSeconds)
		cpuWindow = &value
	}
	windowValid := (view == "cpu" && input.CpuRateWindowSeconds != nil && input.CpuRateWindowSeconds.Valid()) || (view != "cpu" && input.CpuRateWindowSeconds == nil)
	if datasourceUID != "prometheus-main" || !input.View.Valid() || !windowValid || !input.Start.Before(input.End) || input.End.Sub(input.Start) > 6*time.Hour || !input.StepSeconds.Valid() || !input.Mode.Valid() || int(input.End.Sub(input.Start).Seconds())/int(input.StepSeconds)+1 > 1000 {
		return generated.QueryPrometheusOutputSchema{}, runtime.NewError(runtime.SchemaValidationFailed, "Prometheus query input does not match the tool schema", false)
	}
	result, err := s.prometheus.Query(ctx, identity, prometheus.QueryRequest{DatasourceUID: datasourceUID, View: view, CPURateWindowSeconds: cpuWindow, Start: input.Start, End: input.End, StepSeconds: int(input.StepSeconds), Mode: prometheus.QueryMode(input.Mode)})
	if err != nil {
		return generated.QueryPrometheusOutputSchema{}, err
	}
	var output generated.QueryPrometheusOutputSchema
	if err := mapValue(result, &output); err != nil {
		return generated.QueryPrometheusOutputSchema{}, err
	}
	if err := validateQueryOutput(output); err != nil {
		return generated.QueryPrometheusOutputSchema{}, err
	}
	return output, nil
}

func authorize(identity requestcontext.Context) error {
	return runtime.RequirePermission(identity, runtime.PermissionDatasourceQuery)
}

func mapValue(source, destination any) error {
	encoded, err := json.Marshal(source)
	if err != nil {
		return runtime.NewError(runtime.InternalError, "tool output could not be encoded", true)
	}
	if err := json.Unmarshal(encoded, destination); err != nil {
		return runtime.NewError(runtime.SchemaValidationFailed, "tool output does not match its schema", false)
	}
	return nil
}

func validateSearchOutput(output generated.SearchMetricsOutputSchema) error {
	for _, candidate := range output.Candidates {
		if candidate.MetricName == "" || candidate.Description == "" || !candidate.Type.Valid() || len(candidate.Labels) == 0 || candidate.Score < 0 || candidate.Score > 1 || len(candidate.Sources) == 0 {
			return runtime.NewError(runtime.SchemaValidationFailed, "search metrics output does not match the tool schema", false)
		}
	}
	return nil
}

func validateLabelsOutput(output generated.GetMetricLabelsOutputSchema) error {
	if output.MetricName == "" || len(output.LabelNames) == 0 || len(output.SampleValues) == 0 {
		return runtime.NewError(runtime.SchemaValidationFailed, "metric labels output does not match the tool schema", false)
	}
	return nil
}

func validateQueryOutput(output generated.QueryPrometheusOutputSchema) error {
	if !output.Status.Valid() || output.ResultType != "matrix" || !output.Validation.Valid || output.Validation.CanonicalExpression == "" || output.DurationMs < 0 {
		return runtime.NewError(runtime.SchemaValidationFailed, "Prometheus query output does not match the tool schema", false)
	}
	for _, series := range output.Series {
		if series.Name == "" || len(series.Points) == 0 {
			return runtime.NewError(runtime.SchemaValidationFailed, "Prometheus query output has an invalid series", false)
		}
	}
	return nil
}
