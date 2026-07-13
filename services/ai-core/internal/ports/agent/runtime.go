package agent

import (
	"context"

	"mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
)

type EventSink interface {
	Emit(context.Context, dto.AgentEvent) error
}
type Runtime interface {
	Run(context.Context, requestcontext.Context, dto.AgentRunRequest, EventSink) (dto.AgentRunResult, error)
	Resume(context.Context, requestcontext.Context, dto.AgentResumeRequest, EventSink) (dto.AgentRunResult, error)
}
