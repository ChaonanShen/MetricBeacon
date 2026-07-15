package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"mini-torchbearing.local/services/assistant-mcp/internal/adapters/assets/filesystem"
	orderhttp "mini-torchbearing.local/services/assistant-mcp/internal/adapters/orderdemo/http"
	ordermock "mini-torchbearing.local/services/assistant-mcp/internal/adapters/orderdemo/mock"
	prometheushttp "mini-torchbearing.local/services/assistant-mcp/internal/adapters/prometheus/http"
	mockprometheus "mini-torchbearing.local/services/assistant-mcp/internal/adapters/prometheus/mock"
	"mini-torchbearing.local/services/assistant-mcp/internal/adapters/prometheus/registry"
	"mini-torchbearing.local/services/assistant-mcp/internal/namespaces/grafana"
	"mini-torchbearing.local/services/assistant-mcp/internal/namespaces/incident"
	"mini-torchbearing.local/services/assistant-mcp/internal/playbook"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/orderdemo"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/prometheus"
)

// Runtime holds the only inbound HTTP handler exposed by assistant-mcp.
type Runtime struct{ Handler http.Handler }

func Wire(config Config) (*Runtime, error) {
	adapter, ready, err := prometheusAdapter(config)
	if err != nil {
		return nil, err
	}
	schemas, err := loadToolSchemas(config.SchemaDir)
	if err != nil {
		return nil, err
	}
	service := grafana.NewService(adapter)
	handler := grafana.NewHandler(service)
	mcpServer := server.NewMCPServer(
		"mini-torchbearing-assistant-mcp",
		"v1",
		server.WithOutputSchemaValidation(),
	)
	if err := grafana.Register(mcpServer, handler, schemas); err != nil {
		return nil, err
	}
	if config.IncidentEnabled {
		if err := wireIncident(config, mcpServer); err != nil {
			return nil, err
		}
	}
	streamable := server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true))
	mux := http.NewServeMux()
	mux.Handle("/mcp", streamable)
	mux.HandleFunc("/healthz", healthy)
	mux.HandleFunc("/readyz", func(writer http.ResponseWriter, request *http.Request) {
		if err := ready(request.Context()); err != nil {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		healthy(writer, request)
	})
	return &Runtime{Handler: mux}, nil
}

func wireIncident(config Config, mcpServer *server.MCPServer) error {
	assetStore, err := filesystem.New(config.AssetDir, config.AssetSchemaDir)
	if err != nil {
		return fmt.Errorf("load incident assets: %w", err)
	}
	orderPort, err := orderAdapter(config)
	if err != nil {
		return err
	}
	engine, err := playbook.NewEngine(assetStore, orderPort, []byte(config.CheckpointKey), nil)
	if err != nil {
		return err
	}
	schemas, err := loadIncidentToolSchemas(config.IncidentToolSchemaDir)
	if err != nil {
		return err
	}
	return incident.Register(mcpServer, incident.NewHandler(incident.NewService(assetStore, orderPort, engine)), schemas)
}

func orderAdapter(config Config) (orderdemo.Port, error) {
	if config.OrderDriver == "mock" {
		return ordermock.New(ordermock.Scenario(config.OrderMockScenario), time.Now().UTC())
	}
	if config.OrderDriver == "http" {
		return orderhttp.New(config.OrderURL, config.OrderReadToken, config.OrderTimeout)
	}
	return nil, fmt.Errorf("order driver must be mock or http")
}

func prometheusAdapter(config Config) (prometheus.Port, func(context.Context) error, error) {
	if config.PrometheusDatasourceUID != "" && config.PrometheusDatasourceUID != registry.DatasourceUID {
		return nil, nil, fmt.Errorf("Prometheus datasource UID must be %s", registry.DatasourceUID)
	}
	if config.PrometheusDriver == "http" {
		adapter, err := prometheushttp.New(config.PrometheusURL, config.PrometheusTimeout)
		if err != nil {
			return nil, nil, err
		}
		return adapter, adapter.Ready, nil
	}
	if config.PrometheusDriver != "" && config.PrometheusDriver != "mock" {
		return nil, nil, fmt.Errorf("Prometheus driver must be mock or http")
	}
	adapter, err := mockprometheus.New(config.FixtureDir)
	if err != nil {
		return nil, nil, err
	}
	return adapter, func(context.Context) error { return nil }, nil
}

func healthy(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(`{"status":"ok"}`))
}

func loadToolSchemas(directory string) (grafana.ToolSchemas, error) {
	files := map[string][2]string{
		grafana.SearchMetricsTool:   {"search-metrics.input.schema.json", "search-metrics.output.schema.json"},
		grafana.GetMetricLabelsTool: {"get-metric-labels.input.schema.json", "get-metric-labels.output.schema.json"},
		grafana.QueryPrometheusTool: {"query-prometheus.input.schema.json", "query-prometheus.output.schema.json"},
	}
	result := make(grafana.ToolSchemas, len(files))
	for toolName, names := range files {
		input, err := readSchema(filepath.Join(directory, names[0]))
		if err != nil {
			return nil, fmt.Errorf("%s input: %w", toolName, err)
		}
		output, err := readSchema(filepath.Join(directory, names[1]))
		if err != nil {
			return nil, fmt.Errorf("%s output: %w", toolName, err)
		}
		result[toolName] = grafana.Schema{Input: input, Output: output}
	}
	return result, nil
}

func readSchema(path string) (json.RawMessage, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !json.Valid(contents) {
		return nil, fmt.Errorf("schema is not valid JSON")
	}
	return json.RawMessage(contents), nil
}

func loadIncidentToolSchemas(directory string) (incident.ToolSchemas, error) {
	inputs, err := readDefinitions(filepath.Join(directory, "inputs.schema.json"))
	if err != nil {
		return nil, fmt.Errorf("incident input schemas: %w", err)
	}
	outputs, err := readDefinitions(filepath.Join(directory, "outputs.schema.json"))
	if err != nil {
		return nil, fmt.Errorf("incident output schemas: %w", err)
	}
	definitions := map[string][2]string{
		incident.KnowledgeGetTool:      {"AssetGetInput", "AssetOutput"},
		incident.SkillGetTool:          {"AssetGetInput", "AssetOutput"},
		incident.ResolveAlertTool:      {"ResolveAlertInput", "ResolveAlertOutput"},
		incident.StartRunTool:          {"StartRunInput", "StartRunOutput"},
		incident.ResumeRunTool:         {"ResumeRunInput", "ResumeRunOutput"},
		incident.GetRuntimeTool:        {"EmptyInput", "RuntimeOutput"},
		incident.GetQueueTool:          {"EmptyInput", "QueueOutput"},
		incident.GetWorkerTool:         {"EmptyInput", "WorkerOutput"},
		incident.GetPolicyTool:         {"EmptyInput", "PolicyOutput"},
		incident.GetRecentOutcomesTool: {"RecentOutcomesInput", "RecentOutcomesOutput"},
		incident.GetOperationTool:      {"GetOperationInput", "OperationOutput"},
	}
	result := make(incident.ToolSchemas, len(definitions))
	for tool, names := range definitions {
		input, okInput := inputs[names[0]]
		output, okOutput := outputs[names[1]]
		if !okInput || !okOutput {
			return nil, fmt.Errorf("incident tool %s references missing schema definitions", tool)
		}
		result[tool] = incident.Schema{Input: input, Output: output}
	}
	return result, nil
}

func readDefinitions(path string) (map[string]json.RawMessage, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		Definitions map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(contents, &document); err != nil || len(document.Definitions) == 0 {
		return nil, fmt.Errorf("schema definitions are invalid")
	}
	result := make(map[string]json.RawMessage, len(document.Definitions))
	for name, raw := range document.Definitions {
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			return nil, err
		}
		schema["$defs"] = document.Definitions
		encoded, err := json.Marshal(schema)
		if err != nil {
			return nil, err
		}
		result[name] = encoded
	}
	return result, nil
}
