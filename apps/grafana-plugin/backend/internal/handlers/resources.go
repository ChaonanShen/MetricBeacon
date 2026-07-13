// Package handlers is the thin Grafana Resource API -> AI Core proxy.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	identity "mini-torchbearing.local/apps/grafana-plugin/backend/internal/context"
	pluginerrors "mini-torchbearing.local/apps/grafana-plugin/backend/internal/errors"
	generated "mini-torchbearing.local/packages/generated-clients/go"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
)

type ResourceHandler struct {
	// Client is an optional fixed client, primarily useful for isolated tests.
	// Production requests should resolve the endpoint from Grafana App settings.
	Client      generated.ClientInterface
	NewClient   func(endpoint string) (generated.ClientInterface, error)
	MaxResponse int64
}

type appSettings struct {
	AICoreEndpoint string `json:"aiCoreEndpoint"`
}

var _ backend.CallResourceHandler = (*ResourceHandler)(nil)

func (h *ResourceHandler) CallResource(ctx context.Context, request *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	if h == nil {
		return errors.New("resource handler is nil")
	}
	requestContext, ok := identity.FromResourceRequest(request)
	if !ok {
		return h.sendError(sender, http.StatusUnauthorized, "unauthenticated", "Grafana user context is required", "", false)
	}
	client, err := h.clientFor(request)
	if err != nil {
		return h.sendError(sender, http.StatusServiceUnavailable, "dependency_unavailable", "AI Core endpoint is not configured in Grafana App settings", requestContext.RequestID, false)
	}
	path := strings.Trim(strings.SplitN(request.Path, "?", 2)[0], "/")
	switch {
	case request.Method == http.MethodPost && path == "sessions":
		return h.createSession(ctx, client, request, requestContext, sender)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "sessions/") && strings.Count(path, "/") == 1:
		return h.getSession(ctx, client, strings.TrimPrefix(path, "sessions/"), requestContext, sender)
	case request.Method == http.MethodPost && path == "tasks":
		return h.createTask(ctx, client, request, requestContext, sender)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "tasks/") && strings.HasSuffix(path, "/events"):
		return h.streamEvents(ctx, client, request, strings.TrimSuffix(strings.TrimPrefix(path, "tasks/"), "/events"), requestContext, sender)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "tasks/") && strings.Count(path, "/") == 1:
		return h.getTask(ctx, client, strings.TrimPrefix(path, "tasks/"), requestContext, sender)
	default:
		return h.sendError(sender, http.StatusNotFound, "resource_not_found", "Plugin resource was not found", requestContext.RequestID, false)
	}
}

func (h *ResourceHandler) clientFor(request *backend.CallResourceRequest) (generated.ClientInterface, error) {
	if request != nil && request.PluginContext.AppInstanceSettings != nil {
		var settings appSettings
		if err := json.Unmarshal(request.PluginContext.AppInstanceSettings.JSONData, &settings); err != nil {
			return nil, err
		}
		endpoint := strings.TrimSpace(settings.AICoreEndpoint)
		if endpoint == "" || h.NewClient == nil {
			return nil, errors.New("aiCoreEndpoint is missing")
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
			return nil, errors.New("aiCoreEndpoint must be an HTTP(S) URL without credentials")
		}
		return h.NewClient(strings.TrimRight(endpoint, "/"))
	}
	if h.Client != nil {
		return h.Client, nil
	}
	return nil, errors.New("Grafana App settings are unavailable")
}

func (h *ResourceHandler) createSession(ctx context.Context, client generated.ClientInterface, request *backend.CallResourceRequest, identity requestcontext.Context, sender backend.CallResourceResponseSender) error {
	var body generated.CreateSessionJSONRequestBody
	if err := decode(request.Body, &body); err != nil {
		return h.sendError(sender, http.StatusBadRequest, "invalid_argument", "Invalid session request", identity.RequestID, false)
	}
	response, err := client.CreateSession(ctx, sessionParams(identity), body)
	return h.forward(sender, response, err, identity.RequestID, false)
}
func (h *ResourceHandler) getSession(ctx context.Context, client generated.ClientInterface, sessionID string, identity requestcontext.Context, sender backend.CallResourceResponseSender) error {
	response, err := client.GetSession(ctx, sessionID, getSessionParams(identity))
	return h.forward(sender, response, err, identity.RequestID, false)
}
func (h *ResourceHandler) createTask(ctx context.Context, client generated.ClientInterface, request *backend.CallResourceRequest, identity requestcontext.Context, sender backend.CallResourceResponseSender) error {
	key := request.GetHTTPHeader("Idempotency-Key")
	if key == "" {
		return h.sendError(sender, http.StatusBadRequest, "invalid_argument", "Idempotency-Key is required", identity.RequestID, false)
	}
	var body generated.CreateTaskJSONRequestBody
	if err := decode(request.Body, &body); err != nil {
		return h.sendError(sender, http.StatusBadRequest, "invalid_argument", "Invalid task request", identity.RequestID, false)
	}
	response, err := client.CreateTask(ctx, taskParams(identity, key), body)
	return h.forward(sender, response, err, identity.RequestID, false)
}
func (h *ResourceHandler) getTask(ctx context.Context, client generated.ClientInterface, taskID string, identity requestcontext.Context, sender backend.CallResourceResponseSender) error {
	response, err := client.GetTask(ctx, taskID, getTaskParams(identity))
	return h.forward(sender, response, err, identity.RequestID, false)
}
func (h *ResourceHandler) streamEvents(ctx context.Context, client generated.ClientInterface, request *backend.CallResourceRequest, taskID string, identity requestcontext.Context, sender backend.CallResourceResponseSender) error {
	params, err := eventParams(request, identity)
	if err != nil {
		return h.sendError(sender, http.StatusBadRequest, "invalid_argument", "Invalid event replay sequence", identity.RequestID, false)
	}
	response, err := client.StreamTaskEvents(ctx, taskID, params)
	if err != nil {
		return h.forward(sender, nil, err, identity.RequestID, true)
	}
	if response.StatusCode >= 400 {
		return h.forward(sender, response, nil, identity.RequestID, false)
	}
	defer response.Body.Close()
	headers := response.Header
	first := true
	buffer := make([]byte, 16*1024)
	for {
		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			out := &backend.CallResourceResponse{Body: append([]byte(nil), buffer[:count]...)}
			if first {
				out.Status, out.Headers, first = response.StatusCode, copyHeaders(headers), false
			}
			if err := sender.Send(out); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func (h *ResourceHandler) forward(sender backend.CallResourceResponseSender, response *http.Response, requestErr error, requestID string, streaming bool) error {
	if requestErr != nil {
		return h.sendError(sender, http.StatusServiceUnavailable, "dependency_unavailable", "AI Core request failed", requestID, true)
	}
	defer response.Body.Close()
	limit := h.MaxResponse
	if limit <= 0 {
		limit = 1 << 20
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return h.sendError(sender, http.StatusBadGateway, "dependency_unavailable", "AI Core response could not be read", requestID, true)
	}
	if int64(len(body)) > limit {
		return h.sendError(sender, http.StatusBadGateway, "dependency_unavailable", "AI Core response exceeded the configured limit", requestID, true)
	}
	return sender.Send(&backend.CallResourceResponse{Status: response.StatusCode, Headers: copyHeaders(response.Header), Body: body})
}
func (h *ResourceHandler) sendError(sender backend.CallResourceResponseSender, status int, code, message, requestID string, retryable bool) error {
	return sender.Send(&backend.CallResourceResponse{Status: status, Headers: map[string][]string{"Content-Type": {"application/json"}}, Body: pluginerrors.Envelope(code, message, requestID, retryable)})
}

func sessionParams(identity requestcontext.Context) *generated.CreateSessionParams {
	return &generated.CreateSessionParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), XRequestID: identity.RequestID, XTraceID: identity.TraceID}
}
func getSessionParams(identity requestcontext.Context) *generated.GetSessionParams {
	return &generated.GetSessionParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), XRequestID: identity.RequestID, XTraceID: identity.TraceID}
}
func taskParams(identity requestcontext.Context, key string) *generated.CreateTaskParams {
	return &generated.CreateTaskParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), IdempotencyKey: key, XRequestID: identity.RequestID, XTraceID: identity.TraceID}
}
func getTaskParams(identity requestcontext.Context) *generated.GetTaskParams {
	return &generated.GetTaskParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), XRequestID: identity.RequestID, XTraceID: identity.TraceID}
}
func eventParams(request *backend.CallResourceRequest, identity requestcontext.Context) (*generated.StreamTaskEventsParams, error) {
	params := &generated.StreamTaskEventsParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), XRequestID: identity.RequestID, XTraceID: identity.TraceID}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		return nil, err
	}
	if value := parsed.Query().Get("afterSequence"); value != "" {
		number, err := strconv.Atoi(value)
		if err != nil || number < 0 {
			return nil, err
		}
		params.AfterSequence = &number
	}
	if value := request.GetHTTPHeader("Last-Event-ID"); value != "" {
		number, err := strconv.Atoi(value)
		if err != nil || number < 0 {
			return nil, err
		}
		params.LastEventID = &number
	}
	return params, nil
}
func decode(raw []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func copyHeaders(headers http.Header) map[string][]string {
	copied := make(map[string][]string, len(headers))
	for key, values := range headers {
		copied[key] = append([]string(nil), values...)
	}
	return copied
}
