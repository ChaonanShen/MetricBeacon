package tools

import (
	"context"

	"mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
)

type QueryEngine interface {
	Validate(context.Context, requestcontext.Context, dto.ValidateQueryRequest) (dto.QueryValidationResult, error)
	Execute(context.Context, requestcontext.Context, dto.ExecuteQueryRequest) (dto.QueryExecutionResult, error)
}
