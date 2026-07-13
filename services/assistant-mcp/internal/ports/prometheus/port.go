package prometheus

import (
	"context"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
)

// Port is implemented by the deterministic fixture adapter now and a real
// Prometheus HTTP adapter later. No MCP or HTTP SDK type crosses this boundary.
type Port interface {
	SearchMetrics(context.Context, requestcontext.Context, SearchMetricsRequest) (SearchMetricsResult, error)
	GetMetricLabels(context.Context, requestcontext.Context, GetMetricLabelsRequest) (MetricLabelsResult, error)
	Query(context.Context, requestcontext.Context, QueryRequest) (QueryResult, error)
}
