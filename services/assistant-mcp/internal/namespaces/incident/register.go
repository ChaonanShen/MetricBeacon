package incident

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Schema struct {
	Input  json.RawMessage
	Output json.RawMessage
}

type ToolSchemas map[string]Schema

type registration struct {
	name, description string
	handler           server.ToolHandlerFunc
	readOnly          bool
}

const (
	KnowledgeGetTool       = "knowledge.get_document"
	SkillGetTool           = "skills.get_skill"
	ResolveAlertTool       = "playbook.resolve_alert"
	StartRunTool           = "playbook.start_run"
	ResumeRunTool          = "playbook.resume_run"
	GetRuntimeTool         = "order_service.get_runtime"
	GetQueueTool           = "order_service.get_queue_snapshot"
	GetWorkerTool          = "order_service.get_worker_state"
	GetPolicyTool          = "order_service.get_worker_policy"
	GetRecentOutcomesTool  = "order_service.get_recent_outcomes"
	GetOperationTool       = "order_service.get_operation"
	RestoreWorkerTool      = "order_service.restore_worker_concurrency"
	GetRecoveryMetricsTool = "order_service.get_recovery_metrics"
	RunBusinessProbeTool   = "order_service.run_business_probe"
)

func Register(mcpServer *server.MCPServer, handler *Handler, schemas ToolSchemas, remediationEnabled bool) error {
	registrations := []registration{
		{KnowledgeGetTool, "Load one pinned order-service Knowledge document.", handler.KnowledgeGet, true},
		{SkillGetTool, "Load one pinned order-backlog diagnostic Skill.", handler.SkillGet, true},
		{ResolveAlertTool, "Resolve one trusted alert to exactly one pinned Playbook.", handler.ResolveAlert, true},
		{StartRunTool, "Start a bounded Playbook observation and return a signed checkpoint.", handler.StartRun, true},
		{ResumeRunTool, "Resume a signed checkpoint with a strict diagnosis and deterministic prepare policy.", handler.ResumeRun, true},
		{GetRuntimeTool, "Read bounded order-service runtime identity.", handler.GetRuntime, true},
		{GetQueueTool, "Read bounded order queue health.", handler.GetQueue, true},
		{GetWorkerTool, "Read versioned order worker state.", handler.GetWorker, true},
		{GetPolicyTool, "Read the pinned healthy worker policy.", handler.GetPolicy, true},
		{GetRecentOutcomesTool, "Read at most twenty redacted recent order outcomes.", handler.GetRecentOutcomes, true},
		{GetOperationTool, "Read one typed remediation operation receipt.", handler.GetOperation, true},
	}
	if remediationEnabled {
		registrations = append(registrations,
			registration{RestoreWorkerTool, "Execute only an approved, evidence-bound worker restore from zero to two.", handler.RestoreWorkerConcurrency, false},
			registration{GetRecoveryMetricsTool, "Read the fixed registered Prometheus recovery view.", handler.GetRecoveryMetrics, true},
			registration{RunBusinessProbeTool, "Run one fixed bounded real order-processing probe.", handler.RunBusinessProbe, true},
		)
	}
	for _, registration := range registrations {
		schema, ok := schemas[registration.name]
		if !ok || !json.Valid(schema.Input) || !json.Valid(schema.Output) {
			return fmt.Errorf("tool schema %s is unavailable", registration.name)
		}
		tool := mcp.NewToolWithRawSchema(registration.name, registration.description, schema.Input)
		tool.RawOutputSchema = schema.Output
		tool.Annotations.ReadOnlyHint = mcp.ToBoolPtr(registration.readOnly)
		tool.Annotations.DestructiveHint = mcp.ToBoolPtr(false)
		tool.Annotations.IdempotentHint = mcp.ToBoolPtr(true)
		tool.Annotations.OpenWorldHint = mcp.ToBoolPtr(false)
		mcpServer.AddTool(tool, registration.handler)
	}
	return nil
}
