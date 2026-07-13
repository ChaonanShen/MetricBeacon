package dto

import (
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type SearchMetricsRequest struct {
	DatasourceUID, Query string
	Limit                int
}
type MetricCandidate struct {
	MetricName, Type, Description string
	Labels                        []string
	Score                         float64
	Sources                       []MetricSource
}
type MetricSource struct{ Type, Reference string }
type SearchMetricsResult struct{ Candidates []MetricCandidate }
type GetMetricLabelsRequest struct{ DatasourceUID, MetricName string }
type MetricLabelsResult struct {
	MetricName   string
	LabelNames   []string
	SampleValues map[string][]string
}
type ValidateQueryRequest struct{ DatasourceUID, Expression string }
type QueryValidationResult struct {
	Valid                                     bool
	Errors, Warnings, MetricNames, LabelNames []string
}
type ExecuteQueryRequest struct {
	DatasourceUID, Expression string
	TimeRange                 common.AbsoluteTimeRange
	StepSeconds               int
}
type QueryExecutionResult struct {
	Status     string
	Series     []chart.Series
	DurationMS int64
	Warnings   []string
	Validation QueryValidationResult
}
