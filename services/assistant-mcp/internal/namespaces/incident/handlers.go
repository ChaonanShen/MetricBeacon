package incident

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	generated "mini-torchbearing.local/packages/generated-contracts/go"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) KnowledgeGet(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withInput(request, h.service.KnowledgeGet)
}

func (h *Handler) SkillGet(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withInput(request, h.service.SkillGet)
}

func (h *Handler) ResolveAlert(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withInput(request, h.service.ResolveAlert)
}

func (h *Handler) StartRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withContextInput(ctx, request, h.service.StartRun)
}

func (h *Handler) ResumeRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withContextInput(ctx, request, h.service.ResumeRun)
}

func (h *Handler) GetRuntime(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withoutInput(ctx, request, h.service.GetRuntime)
}

func (h *Handler) GetQueue(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withoutInput(ctx, request, h.service.GetQueue)
}

func (h *Handler) GetWorker(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withoutInput(ctx, request, h.service.GetWorker)
}

func (h *Handler) GetPolicy(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withoutInput(ctx, request, h.service.GetPolicy)
}

func (h *Handler) GetRecentOutcomes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withContextInput(ctx, request, h.service.GetRecentOutcomes)
}

func (h *Handler) GetOperation(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return withContextInput(ctx, request, h.service.GetOperation)
}

func withInput[I, O any](request mcp.CallToolRequest, operation func(requestcontext.Context, I) (O, error)) (*mcp.CallToolResult, error) {
	identity, err := runtime.RequestContextFromHeaders(request.Header)
	if err != nil {
		return errorResult(err, request.Header), nil
	}
	var input I
	if err := decodeArguments(request, &input); err != nil {
		return errorResult(err, request.Header), nil
	}
	output, err := operation(identity, input)
	if err != nil {
		return errorResult(err, request.Header), nil
	}
	return mcp.NewToolResultStructuredOnly(output), nil
}

func withContextInput[I, O any](ctx context.Context, request mcp.CallToolRequest, operation func(context.Context, requestcontext.Context, I) (O, error)) (*mcp.CallToolResult, error) {
	identity, err := runtime.RequestContextFromHeaders(request.Header)
	if err != nil {
		return errorResult(err, request.Header), nil
	}
	var input I
	if err := decodeArguments(request, &input); err != nil {
		return errorResult(err, request.Header), nil
	}
	output, err := operation(ctx, identity, input)
	if err != nil {
		return errorResult(err, request.Header), nil
	}
	return mcp.NewToolResultStructuredOnly(output), nil
}

func withoutInput[O any](ctx context.Context, request mcp.CallToolRequest, operation func(context.Context, requestcontext.Context) (O, error)) (*mcp.CallToolResult, error) {
	return withContextInput(ctx, request, func(ctx context.Context, identity requestcontext.Context, input generated.EmptyInput) (O, error) {
		if len(input) != 0 {
			var zero O
			return zero, invalidInput()
		}
		return operation(ctx, identity)
	})
}

func decodeArguments(request mcp.CallToolRequest, destination any) error {
	raw := request.Params.RawArguments
	if len(raw) == 0 {
		var err error
		raw, err = json.Marshal(request.Params.Arguments)
		if err != nil {
			return invalidInput()
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return invalidInput()
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return invalidInput()
	}
	return nil
}

func errorResult(err error, headers http.Header) *mcp.CallToolResult {
	toolError, ok := err.(*runtime.ToolError)
	if !ok {
		toolError = runtime.NewError(runtime.InternalError, "tool execution failed", true)
	}
	envelope := struct {
		Code      runtime.ErrorCode `json:"code"`
		Message   string            `json:"message"`
		Retryable bool              `json:"retryable"`
		RequestID string            `json:"requestId"`
	}{Code: toolError.Code, Message: toolError.Message, Retryable: toolError.Retryable, RequestID: headers.Get(runtime.HeaderRequestID)}
	encoded, _ := json.Marshal(envelope)
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(encoded)}}, StructuredContent: envelope, IsError: true}
}
