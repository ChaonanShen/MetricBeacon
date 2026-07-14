package agent

import (
	"context"
	"time"

	"mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
)

type IntentStatus string

const (
	IntentPlanned     IntentStatus = "planned"
	IntentUnsupported IntentStatus = "unsupported"
)

type IntentPlanRequest struct {
	Message string
	History []dto.ConversationMessage
}

type IntentPlan struct {
	Status        IntentStatus
	Views         []string
	RangeDuration *time.Duration
	StepSeconds   *int
}

type IntentPlanner interface {
	Plan(context.Context, requestcontext.Context, IntentPlanRequest) (IntentPlan, error)
}

type EventSink interface {
	Emit(context.Context, dto.AgentEvent) error
}
type Runtime interface {
	Run(context.Context, requestcontext.Context, dto.AgentRunRequest, EventSink) (dto.AgentRunResult, error)
	Resume(context.Context, requestcontext.Context, dto.AgentResumeRequest, EventSink) (dto.AgentRunResult, error)
}
