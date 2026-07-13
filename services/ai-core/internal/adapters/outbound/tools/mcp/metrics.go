package mcp

import (
	"context"
	"encoding/json"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

type MetricCatalogAdapter struct{ gateway tools.Gateway }

var _ tools.MetricCatalog = (*MetricCatalogAdapter)(nil)

func NewMetricCatalogAdapter(gateway tools.Gateway) *MetricCatalogAdapter {
	return &MetricCatalogAdapter{gateway: gateway}
}

func (a *MetricCatalogAdapter) SearchMetrics(ctx context.Context, identity requestcontext.Context, request dto.SearchMetricsRequest) (dto.SearchMetricsResult, error) {
	arguments, _ := json.Marshal(struct {
		DatasourceUID string `json:"datasourceUid"`
		Query         string `json:"query"`
		Limit         int    `json:"limit"`
	}{request.DatasourceUID, request.Query, request.Limit})
	result, err := a.gateway.CallTool(ctx, identity, tools.Call{Name: "grafana.search_metrics", Version: "v1", Arguments: arguments})
	if err != nil {
		return dto.SearchMetricsResult{}, err
	}
	var output dto.SearchMetricsResult
	if err := json.Unmarshal(result.Content, &output); err != nil {
		return dto.SearchMetricsResult{}, common.NewError(common.SchemaValidationFailed, "MCP search result does not match the contract", false)
	}
	return output, nil
}

func (a *MetricCatalogAdapter) GetMetricLabels(ctx context.Context, identity requestcontext.Context, request dto.GetMetricLabelsRequest) (dto.MetricLabelsResult, error) {
	arguments, _ := json.Marshal(struct {
		DatasourceUID string `json:"datasourceUid"`
		MetricName    string `json:"metricName"`
	}{request.DatasourceUID, request.MetricName})
	result, err := a.gateway.CallTool(ctx, identity, tools.Call{Name: "grafana.get_metric_labels", Version: "v1", Arguments: arguments})
	if err != nil {
		return dto.MetricLabelsResult{}, err
	}
	var output dto.MetricLabelsResult
	if err := json.Unmarshal(result.Content, &output); err != nil {
		return dto.MetricLabelsResult{}, common.NewError(common.SchemaValidationFailed, "MCP labels result does not match the contract", false)
	}
	return output, nil
}
