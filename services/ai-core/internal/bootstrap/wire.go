package bootstrap

import (
	"context"
	"fmt"

	deepseekmodel "github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cohesion-org/deepseek-go"

	approvalevidence "mini-torchbearing.local/packages/approval-evidence-go"
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
	"mini-torchbearing.local/services/ai-core/internal/application/approvals"
	"mini-torchbearing.local/services/ai-core/internal/application/commands"
	"mini-torchbearing.local/services/ai-core/internal/application/incidents"
	"mini-torchbearing.local/services/ai-core/internal/application/workflows"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
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
	var planner agent.IntentPlanner = mock.Planner{}
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
			MaxTokens:          512,
			Temperature:        0.01,
			Timeout:            config.ModelTimeout,
			ResponseFormatType: deepseekmodel.ResponseFormatTypeJSONObject,
			ThinkingConfig:     &deepseek.ThinkingConfig{Type: "disabled"},
		})
		if err != nil {
			_ = store.Close()
			return nil, common.NewError(common.AdapterNotConfigured, "DeepSeek model configuration is invalid", false)
		}
		planner, err = einoagent.NewPlanner(chatModel, nodeProfile, config.AgentTimeout)
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
	commandService := commands.New(store, notifier, workflow, generator, clock, planner)
	api := httpapi.API{Commands: commandService, Store: store, Notifier: notifier}
	if config.IncidentWebhookSecret != "" {
		evidence, err := approvalevidence.New([]byte(config.ApprovalEvidenceKey))
		if err != nil {
			_ = store.Close()
			return nil, common.NewError(common.AdapterNotConfigured, "Incident ApprovalEvidence configuration is invalid", false)
		}
		incidentTools := mcpadapter.NewIncidentToolset(gateway)
		incidentWorkflow := workflows.RunIncidentWorkflow{Store: store, Notifier: notifier, Toolset: incidentTools, IDs: generator, Clock: clock}
		remediationTools := mcpadapter.NewIncidentRemediationToolset(gateway)
		remediationWorkflow := workflows.RunRemediationWorkflow{Store: store, Notifier: notifier, Toolset: remediationTools, Evidence: evidence, IDs: generator, Clock: clock}
		incidentService := incidents.New(incidents.Config{TenantID: config.IncidentTenantID, OrgID: fmt.Sprint(config.IncidentOrgID), ActorID: config.IncidentActorID}, store, notifier, incidentTools, incidentWorkflow, generator, clock)
		if err := incidentService.Recover(ctx); err != nil {
			_ = store.Close()
			return nil, err
		}
		recoveryIdentity := func(item task.AnalysisTask) requestcontext.Context {
			return requestcontext.Context{TenantID: item.TenantID, OrgID: fmt.Sprint(config.IncidentOrgID), UserID: "system:incident-recovery", Roles: []string{"IncidentAgent"}, Permissions: []string{"incidents:remediate"}, RequestID: "incident-recovery", TraceID: "incident-recovery"}
		}
		if err := remediationWorkflow.Recover(ctx, recoveryIdentity); err != nil {
			_ = store.Close()
			return nil, err
		}
		api.Approvals = approvals.New(store, notifier, remediationWorkflow, generator, clock)
		api.Incidents = incidentService
		api.AlertIngress = httpapi.AlertIngressConfig{SourceID: config.IncidentAlertSource, OrgID: config.IncidentOrgID, HMACSecret: config.IncidentWebhookSecret, MaxClockSkew: config.IncidentAlertMaxSkew, CurrentTime: clock.Now}
	}
	api.Readiness = func(checkCtx context.Context) error {
		identity := requestcontext.Context{TenantID: "readiness", OrgID: "0", UserID: "readiness", Roles: []string{"Admin"}, Permissions: []string{"datasources:query"}, RequestID: "readyz", TraceID: "readyz"}
		descriptors, err := gateway.ListTools(checkCtx, identity, tools.Filter{Namespace: "grafana"})
		if err != nil {
			return err
		}
		if len(descriptors) != 3 {
			return common.NewError(common.DependencyUnavailable, "assistant-mcp did not expose the required tools", true)
		}
		if config.IncidentWebhookSecret != "" {
			identity.TenantID, identity.OrgID, identity.UserID = config.IncidentTenantID, fmt.Sprint(config.IncidentOrgID), config.IncidentActorID
			identity.Permissions = []string{"incidents:diagnose", "incidents:remediate"}
			if err := validateIncidentRemediationProfile(checkCtx, gateway, identity); err != nil {
				return err
			}
		}
		return nil
	}
	return &Application{Handler: api, close: store.Close}, nil
}

func validateIncidentRemediationProfile(ctx context.Context, gateway tools.Gateway, identity requestcontext.Context) error {
	for namespace, expected := range map[string]int{"knowledge": 1, "skills": 1, "playbook": 3, "order_service": 9} {
		profile, err := gateway.ListTools(ctx, identity, tools.Filter{Namespace: namespace})
		if err != nil || len(profile) != expected {
			return common.NewError(common.DependencyUnavailable, "assistant-mcp Incident remediation profile is incomplete", true)
		}
	}
	return nil
}
