package grafana

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	generated "mini-torchbearing.local/packages/generated-contracts/go"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) SearchMetrics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	identity, err := runtime.RequestContextFromHeaders(request.Header)
	if err != nil {
		return errorResult(err, request.Header), nil
	}
	var input generated.SearchMetricsInputSchema
	if err := decodeArguments(request, &input); err != nil {
		return errorResult(err, request.Header), nil
	}
	output, err := h.service.SearchMetrics(ctx, identity, input)
	if err != nil {
		return errorResult(err, request.Header), nil
	}
	return mcp.NewToolResultStructuredOnly(output), nil
}

func (h *Handler) GetMetricLabels(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	identity, err := runtime.RequestContextFromHeaders(request.Header)
	if err != nil {
		return errorResult(err, request.Header), nil
	}
	var input generated.GetMetricLabelsInputSchema
	if err := decodeArguments(request, &input); err != nil {
		return errorResult(err, request.Header), nil
	}
	output, err := h.service.GetMetricLabels(ctx, identity, input)
	if err != nil {
		return errorResult(err, request.Header), nil
	}
	return mcp.NewToolResultStructuredOnly(output), nil
}

func (h *Handler) QueryPrometheus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	identity, err := runtime.RequestContextFromHeaders(request.Header)
	if err != nil {
		return errorResult(err, request.Header), nil
	}
	var input generated.QueryPrometheusInputSchema
	if err := decodeArguments(request, &input); err != nil {
		return errorResult(err, request.Header), nil
	}
	output, err := h.service.QueryPrometheus(ctx, identity, input)
	if err != nil {
		return errorResult(err, request.Header), nil
	}
	return mcp.NewToolResultStructuredOnly(output), nil
}

func decodeArguments(request mcp.CallToolRequest, destination any) error {
	raw := request.Params.RawArguments
	if len(raw) == 0 {
		var err error
		raw, err = json.Marshal(request.Params.Arguments)
		if err != nil {
			return runtime.NewError(runtime.SchemaValidationFailed, "tool arguments could not be decoded", false)
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return runtime.NewError(runtime.SchemaValidationFailed, "tool arguments do not match the schema", false)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return runtime.NewError(runtime.SchemaValidationFailed, "tool arguments do not match the schema", false)
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
