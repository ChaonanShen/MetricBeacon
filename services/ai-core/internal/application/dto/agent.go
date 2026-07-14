package dto

import (
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type AgentRunRequest struct {
	TaskID        string
	SessionID     string
	UserMessage   string
	DatasourceUID string
	TimeRange     common.AbsoluteTimeRange
	History       []ConversationMessage
}

type ConversationMessage struct {
	Role    string
	Content string
}

type ChartProposal struct {
	Key           string
	Title         string
	Visualization string
	Unit          string
	Query         chart.QuerySpec
	Execution     QueryExecutionResult
}

type AgentRunResult struct {
	AssistantText string
	Proposals     []ChartProposal
}
type AgentResumeRequest struct{ TaskID string }
type AgentEvent struct {
	Type         string
	SourceCallID string
	Payload      any
}
