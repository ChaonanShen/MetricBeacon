package mcp

import (
	"context"
	"encoding/json"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

type QueryEngineAdapter struct{ gateway tools.Gateway }

var _ tools.QueryEngine = (*QueryEngineAdapter)(nil)

func NewQueryEngineAdapter(gateway tools.Gateway) *QueryEngineAdapter {
	return &QueryEngineAdapter{gateway: gateway}
}

func (a *QueryEngineAdapter) Validate(ctx context.Context, identity requestcontext.Context, request dto.ValidateQueryRequest) (dto.QueryValidationResult, error) {
	now := time.Now().UTC()
	output, err := a.query(ctx, identity, request.DatasourceUID, request.Expression, now.Add(-time.Minute), now, 60, "validate")
	return output.Validation, err
}

func (a *QueryEngineAdapter) Execute(ctx context.Context, identity requestcontext.Context, request dto.ExecuteQueryRequest) (dto.QueryExecutionResult, error) {
	return a.query(ctx, identity, request.DatasourceUID, request.Expression, request.TimeRange.From, request.TimeRange.To, request.StepSeconds, "execute")
}

func (a *QueryEngineAdapter) query(ctx context.Context, identity requestcontext.Context, datasourceUID, expression string, start, end time.Time, stepSeconds int, mode string) (dto.QueryExecutionResult, error) {
	arguments, _ := json.Marshal(struct {
		DatasourceUID string    `json:"datasourceUid"`
		Expression    string    `json:"expression"`
		Start         time.Time `json:"start"`
		End           time.Time `json:"end"`
		StepSeconds   int       `json:"stepSeconds"`
		Mode          string    `json:"mode"`
	}{datasourceUID, expression, start.UTC(), end.UTC(), stepSeconds, mode})
	result, err := a.gateway.CallTool(ctx, identity, tools.Call{Name: "grafana.query_prometheus", Version: "v1", Arguments: arguments})
	if err != nil {
		return dto.QueryExecutionResult{}, err
	}
	var wire struct {
		Validation dto.QueryValidationResult `json:"validation"`
		Status     string                    `json:"status"`
		ResultType string                    `json:"resultType"`
		Series     []struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
			Points []struct {
				Timestamp time.Time `json:"timestamp"`
				Value     float64   `json:"value"`
			} `json:"points"`
		} `json:"series"`
		DurationMS int64    `json:"durationMs"`
		Warnings   []string `json:"warnings"`
	}
	if err := json.Unmarshal(result.Content, &wire); err != nil || wire.ResultType != "matrix" || (wire.Status != "success" && wire.Status != "failed") {
		return dto.QueryExecutionResult{}, common.NewError(common.SchemaValidationFailed, "MCP query result does not match the contract", false)
	}
	output := dto.QueryExecutionResult{Status: wire.Status, DurationMS: wire.DurationMS, Warnings: wire.Warnings, Validation: wire.Validation, Series: make([]chart.Series, len(wire.Series))}
	for i, series := range wire.Series {
		output.Series[i] = chart.Series{Name: series.Name, Labels: series.Labels, Points: make([]chart.Point, len(series.Points))}
		for j, point := range series.Points {
			output.Series[i].Points[j] = chart.Point{Timestamp: point.Timestamp.UTC(), Value: point.Value}
		}
	}
	return output, nil
}
