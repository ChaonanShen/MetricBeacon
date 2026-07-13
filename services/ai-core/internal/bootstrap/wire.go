package bootstrap

import (
	"context"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	httpapi "mini-torchbearing.local/services/ai-core/internal/adapters/inbound/http"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/mock"
	clockadapter "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/clocks"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/events/inmemory"
	idadapter "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/ids"
	storage "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/storage/sqlite"
	mcpadapter "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/tools/mcp"
	"mini-torchbearing.local/services/ai-core/internal/application/commands"
	"mini-torchbearing.local/services/ai-core/internal/application/workflows"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
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
	store, err := storage.Open(ctx, config.SQLitePath)
	if err != nil {
		return nil, err
	}
	notifier := inmemory.New()
	clock := clockadapter.NewSystem()
	generator := idadapter.New()
	gateway := mcpadapter.NewGateway(config.AssistantMCPEndpoint, 5*time.Second)
	runtime := mock.New(mcpadapter.NewMetricCatalogAdapter(gateway), mcpadapter.NewQueryEngineAdapter(gateway))
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
