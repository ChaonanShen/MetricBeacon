package tools

import (
	"context"

	"mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
)

type MetricCatalog interface {
	SearchMetrics(context.Context, requestcontext.Context, dto.SearchMetricsRequest) (dto.SearchMetricsResult, error)
	GetMetricLabels(context.Context, requestcontext.Context, dto.GetMetricLabelsRequest) (dto.MetricLabelsResult, error)
}
