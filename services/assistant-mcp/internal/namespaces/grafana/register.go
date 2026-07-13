package grafana

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ToolSchemas map[string]Schema

type Schema struct {
	Input  json.RawMessage
	Output json.RawMessage
}

const (
	SearchMetricsTool   = "grafana.search_metrics"
	GetMetricLabelsTool = "grafana.get_metric_labels"
	QueryPrometheusTool = "grafana.query_prometheus"
)

func Register(mcpServer *server.MCPServer, handler *Handler, schemas ToolSchemas) error {
	for _, registration := range []struct {
		name        string
		description string
		handler     server.ToolHandlerFunc
	}{
		{SearchMetricsTool, "Search read-only Prometheus metric metadata.", handler.SearchMetrics},
		{GetMetricLabelsTool, "Get read-only label names and sample values for a metric.", handler.GetMetricLabels},
		{QueryPrometheusTool, "Validate or execute a read-only PromQL range query.", handler.QueryPrometheus},
	} {
		schema, ok := schemas[registration.name]
		if !ok || !json.Valid(schema.Input) || !json.Valid(schema.Output) {
			return fmt.Errorf("tool schema %s is unavailable", registration.name)
		}
		tool := mcp.NewToolWithRawSchema(registration.name, registration.description, schema.Input)
		tool.RawOutputSchema = schema.Output
		tool.Annotations.ReadOnlyHint = mcp.ToBoolPtr(true)
		tool.Annotations.DestructiveHint = mcp.ToBoolPtr(false)
		tool.Annotations.IdempotentHint = mcp.ToBoolPtr(true)
		tool.Annotations.OpenWorldHint = mcp.ToBoolPtr(false)
		mcpServer.AddTool(tool, registration.handler)
	}
	return nil
}
