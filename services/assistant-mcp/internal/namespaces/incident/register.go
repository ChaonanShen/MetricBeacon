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

const (
	KnowledgeGetTool      = "knowledge.get_document"
	SkillGetTool          = "skills.get_skill"
	ResolveAlertTool      = "playbook.resolve_alert"
	StartRunTool          = "playbook.start_run"
	ResumeRunTool         = "playbook.resume_run"
	GetRuntimeTool        = "order_service.get_runtime"
	GetQueueTool          = "order_service.get_queue_snapshot"
	GetWorkerTool         = "order_service.get_worker_state"
	GetPolicyTool         = "order_service.get_worker_policy"
	GetRecentOutcomesTool = "order_service.get_recent_outcomes"
	GetOperationTool      = "order_service.get_operation"
)

func Register(mcpServer *server.MCPServer, handler *Handler, schemas ToolSchemas) error {
	registrations := []struct {
		name, description string
		handler           server.ToolHandlerFunc
	}{
		{KnowledgeGetTool, "Load one pinned order-service Knowledge document.", handler.KnowledgeGet},
		{SkillGetTool, "Load one pinned order-backlog diagnostic Skill.", handler.SkillGet},
		{ResolveAlertTool, "Resolve one trusted alert to exactly one pinned Playbook.", handler.ResolveAlert},
		{StartRunTool, "Start a bounded Playbook observation and return a signed checkpoint.", handler.StartRun},
		{ResumeRunTool, "Resume a signed checkpoint with a strict diagnosis and deterministic prepare policy.", handler.ResumeRun},
		{GetRuntimeTool, "Read bounded order-service runtime identity.", handler.GetRuntime},
		{GetQueueTool, "Read bounded order queue health.", handler.GetQueue},
		{GetWorkerTool, "Read versioned order worker state.", handler.GetWorker},
		{GetPolicyTool, "Read the pinned healthy worker policy.", handler.GetPolicy},
		{GetRecentOutcomesTool, "Read at most twenty redacted recent order outcomes.", handler.GetRecentOutcomes},
		{GetOperationTool, "Read one typed remediation operation receipt.", handler.GetOperation},
	}
	for _, registration := range registrations {
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
