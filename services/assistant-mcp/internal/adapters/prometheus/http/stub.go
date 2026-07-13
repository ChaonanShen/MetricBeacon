// Package http reserves the real Prometheus HTTP adapter replacement point.
package http

import (
	"context"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/prometheus"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

type Adapter struct{ Endpoint string }

var _ prometheus.Port = (*Adapter)(nil)

func New(endpoint string) *Adapter { return &Adapter{Endpoint: endpoint} }

func (a *Adapter) SearchMetrics(context.Context, requestcontext.Context, prometheus.SearchMetricsRequest) (prometheus.SearchMetricsResult, error) {
	return prometheus.SearchMetricsResult{}, runtime.NewError(runtime.NotImplemented, "real Prometheus adapter is not implemented", false)
}
func (a *Adapter) GetMetricLabels(context.Context, requestcontext.Context, prometheus.GetMetricLabelsRequest) (prometheus.MetricLabelsResult, error) {
	return prometheus.MetricLabelsResult{}, runtime.NewError(runtime.NotImplemented, "real Prometheus adapter is not implemented", false)
}
func (a *Adapter) Query(context.Context, requestcontext.Context, prometheus.QueryRequest) (prometheus.QueryResult, error) {
	return prometheus.QueryResult{}, runtime.NewError(runtime.NotImplemented, "real Prometheus adapter is not implemented", false)
}
