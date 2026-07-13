// Package mcp implements the AI Core ToolGateway over real Streamable HTTP.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpprotocol "github.com/mark3labs/mcp-go/mcp"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

type Gateway struct {
	endpoint string
	timeout  time.Duration
}

var _ tools.Gateway = (*Gateway)(nil)

func NewGateway(endpoint string, timeout time.Duration) *Gateway {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Gateway{endpoint: endpoint, timeout: timeout}
}

func (g *Gateway) ListTools(ctx context.Context, identity requestcontext.Context, filter tools.Filter) ([]tools.Descriptor, error) {
	client, callContext, cancel, err := g.newClient(ctx, identity)
	if err != nil {
		return nil, err
	}
	defer cancel()
	result, err := client.ListTools(callContext, mcpprotocol.ListToolsRequest{})
	if err != nil {
		return nil, mapMCPError(err)
	}
	descriptors := make([]tools.Descriptor, 0, len(result.Tools))
	for _, tool := range result.Tools {
		if filter.Namespace != "" && !strings.HasPrefix(tool.Name, filter.Namespace+".") {
			continue
		}
		descriptors = append(descriptors, tools.Descriptor{Name: tool.Name, Version: "v1"})
	}
	return descriptors, nil
}

func (g *Gateway) CallTool(ctx context.Context, identity requestcontext.Context, call tools.Call) (tools.Result, error) {
	if call.Name == "" || call.Version != "v1" || !json.Valid(call.Arguments) {
		return tools.Result{}, common.NewError(common.InvalidArgument, "MCP tool call is invalid", false)
	}
	var arguments any
	if err := json.Unmarshal(call.Arguments, &arguments); err != nil {
		return tools.Result{}, common.NewError(common.SchemaValidationFailed, "MCP tool arguments are invalid", false)
	}
	client, callContext, cancel, err := g.newClient(ctx, identity)
	if err != nil {
		return tools.Result{}, err
	}
	defer cancel()
	result, err := client.CallTool(callContext, mcpprotocol.CallToolRequest{Params: mcpprotocol.CallToolParams{Name: call.Name, Arguments: arguments}})
	if err != nil {
		return tools.Result{}, mapMCPError(err)
	}
	if result.IsError {
		return tools.Result{}, mapToolResultError(result)
	}
	content, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return tools.Result{}, common.NewError(common.SchemaValidationFailed, "MCP tool returned an invalid structured result", false)
	}
	return tools.Result{Content: content}, nil
}

func (g *Gateway) newClient(parent context.Context, identity requestcontext.Context) (*client.Client, context.Context, context.CancelFunc, error) {
	if g.endpoint == "" {
		return nil, nil, func() {}, common.NewError(common.AdapterNotConfigured, "assistant-mcp endpoint is not configured", false)
	}
	ctx, cancel := context.WithTimeout(parent, g.timeout)
	transportClient, err := transport.NewStreamableHTTP(g.endpoint, transport.WithHTTPHeaders(headers(identity)))
	if err != nil {
		cancel()
		return nil, nil, func() {}, common.NewError(common.DependencyUnavailable, "assistant-mcp transport is unavailable", true)
	}
	client := client.NewClient(transportClient)
	request := mcpprotocol.InitializeRequest{}
	request.Params.ProtocolVersion = mcpprotocol.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = mcpprotocol.Implementation{Name: "mini-torchbearing-ai-core", Version: "v1"}
	request.Params.Capabilities = mcpprotocol.ClientCapabilities{}
	if _, err := client.Initialize(ctx, request); err != nil {
		cancel()
		_ = client.Close()
		return nil, nil, func() {}, mapMCPError(err)
	}
	return client, ctx, func() { _ = client.Close(); cancel() }, nil
}

func headers(identity requestcontext.Context) map[string]string {
	return map[string]string{
		"X-MTB-Tenant-ID":   identity.TenantID,
		"X-MTB-Org-ID":      identity.OrgID,
		"X-MTB-User-ID":     identity.UserID,
		"X-MTB-Roles":       strings.Join(identity.Roles, ","),
		"X-MTB-Permissions": strings.Join(identity.Permissions, ","),
		"X-Request-ID":      identity.RequestID,
		"X-Trace-ID":        identity.TraceID,
	}
}

func mapMCPError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return common.NewError(common.ToolTimeout, "assistant-mcp tool call timed out", true)
	}
	return common.NewError(common.DependencyUnavailable, "assistant-mcp tool call failed", true)
}

func mapToolResultError(result *mcpprotocol.CallToolResult) error {
	encoded, _ := json.Marshal(result.StructuredContent)
	var envelope struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(encoded, &envelope) == nil {
		switch common.ErrorCode(envelope.Code) {
		case common.InvalidArgument, common.Unauthenticated, common.PermissionDenied, common.ToolNotSupported, common.SchemaValidationFailed, common.NotImplemented:
			return common.NewError(common.ErrorCode(envelope.Code), "assistant-mcp rejected the tool call", false)
		}
	}
	return common.NewError(common.DependencyUnavailable, "assistant-mcp tool execution failed", true)
}
