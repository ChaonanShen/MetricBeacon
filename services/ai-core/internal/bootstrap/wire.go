package bootstrap

import (
	"context"

	deepseekmodel "github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cohesion-org/deepseek-go"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	httpapi "mini-torchbearing.local/services/ai-core/internal/adapters/inbound/http"
	einoagent "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/eino"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/mock"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/profile"
	clockadapter "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/clocks"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/events/inmemory"
	idadapter "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/ids"
	storage "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/storage/sqlite"
	mcpadapter "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/tools/mcp"
	"mini-torchbearing.local/services/ai-core/internal/application/commands"
	"mini-torchbearing.local/services/ai-core/internal/application/workflows"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

type Application struct {
	Handler httpapi.API
	close   func() error
}

func (a *Application) Close() error {
	if a == nil || a.close == nil {
		return nil
	}
	return a.close()
}

func New(ctx context.Context, config Config) (*Application, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	store, err := storage.Open(ctx, config.SQLitePath)
	if err != nil {
		return nil, err
	}
	notifier := inmemory.New()
	clock := clockadapter.NewSystem()
	generator := idadapter.New()
	gateway := mcpadapter.NewGateway(config.AssistantMCPEndpoint, config.MCPToolTimeout)
	catalog := mcpadapter.NewMetricCatalogAdapter(gateway)
	queries := mcpadapter.NewQueryEngineAdapter(gateway)
	var runtime agent.Runtime = mock.New(catalog, queries)
	if config.AgentDriver == "eino" {
		nodeProfile, err := profile.Load(config.AgentProfilePath)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		chatModel, err := deepseekmodel.NewChatModel(ctx, &deepseekmodel.ChatModelConfig{
			APIKey:             config.DeepSeekAPIKey,
			BaseURL:            config.DeepSeekBaseURL,
			Model:              config.DeepSeekModel,
			MaxTokens:          2048,
			Temperature:        0.1,
			Timeout:            config.ModelTimeout,
			ResponseFormatType: deepseekmodel.ResponseFormatTypeJSONObject,
			ThinkingConfig:     &deepseek.ThinkingConfig{Type: "disabled"},
		})
		if err != nil {
			_ = store.Close()
			return nil, common.NewError(common.AdapterNotConfigured, "DeepSeek model configuration is invalid", false)
		}
		runtime, err = einoagent.New(chatModel, catalog, queries, nodeProfile, einoagent.Limits{MaxIterations: config.AgentMaxIterations, MaxToolCalls: config.AgentMaxToolCalls, Timeout: config.AgentTimeout})
		if err != nil {
			_ = store.Close()
			return nil, err
		}
	}
	workflow := workflows.RunAnalysisWorkflow{Store: store, Notifier: notifier, Runtime: runtime, IDs: generator, Clock: clock}
	if err := workflow.RecoverInterrupted(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	commandService := commands.New(store, notifier, workflow, generator, clock)
	api := httpapi.API{Commands: commandService, Store: store, Notifier: notifier, Readiness: func(checkCtx context.Context) error {
		identity := requestcontext.Context{TenantID: "readiness", OrgID: "0", UserID: "readiness", Roles: []string{"Admin"}, Permissions: []string{"datasources:query"}, RequestID: "readyz", TraceID: "readyz"}
		descriptors, err := gateway.ListTools(checkCtx, identity, tools.Filter{Namespace: "grafana"})
		if err != nil {
			return err
		}
		if len(descriptors) != 3 {
			return common.NewError(common.DependencyUnavailable, "assistant-mcp did not expose the required tools", true)
		}
		return nil
	}}
	return &Application{Handler: api, close: store.Close}, nil
}
