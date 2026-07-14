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
	Client    generated.ClientInterface
	NewClient func(endpoint string) (generated.ClientInterface, error)
	// NewStreamClient creates a client without a global timeout. It is used only
	// for the SSE follow endpoint, whose lifecycle is request-context driven.
	NewStreamClient func(endpoint string) (generated.ClientInterface, error)
	MaxResponse     int64
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
	streaming := request.Method == http.MethodGet && strings.HasPrefix(strings.Trim(strings.SplitN(request.Path, "?", 2)[0], "/"), "tasks/") && strings.HasSuffix(strings.Trim(strings.SplitN(request.Path, "?", 2)[0], "/"), "/events")
	client, err := h.clientFor(request, streaming)
	if err != nil {
		return h.sendError(sender, http.StatusServiceUnavailable, "dependency_unavailable", "AI Core endpoint is not configured in Grafana App settings", requestContext.RequestID, false)
	}
	path := strings.Trim(strings.SplitN(request.Path, "?", 2)[0], "/")
	switch {
	case request.Method == http.MethodPost && path == "sessions":
		return h.createSession(ctx, client, request, requestContext, sender)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "sessions/") && strings.Count(path, "/") == 1:
		return h.getSession(ctx, client, strings.TrimPrefix(path, "sessions/"), requestContext, sender)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "sessions/") && strings.HasSuffix(path, "/messages"):
		return h.listMessages(ctx, client, request, strings.TrimSuffix(strings.TrimPrefix(path, "sessions/"), "/messages"), requestContext, sender)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "sessions/") && strings.HasSuffix(path, "/tasks"):
		return h.listTasks(ctx, client, request, strings.TrimSuffix(strings.TrimPrefix(path, "sessions/"), "/tasks"), requestContext, sender)
	case request.Method == http.MethodPost && path == "tasks":
		return h.createTask(ctx, client, request, requestContext, sender)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "tasks/") && strings.HasSuffix(path, "/events/replay"):
		return h.replayEvents(ctx, client, request, strings.TrimSuffix(strings.TrimPrefix(path, "tasks/"), "/events/replay"), requestContext, sender)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "tasks/") && strings.HasSuffix(path, "/events"):
		return h.streamEvents(ctx, client, request, strings.TrimSuffix(strings.TrimPrefix(path, "tasks/"), "/events"), requestContext, sender)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "tasks/") && strings.Count(path, "/") == 1:
		return h.getTask(ctx, client, strings.TrimPrefix(path, "tasks/"), requestContext, sender)
	default:
		return h.sendError(sender, http.StatusNotFound, "resource_not_found", "Plugin resource was not found", requestContext.RequestID, false)
	}
}

func (h *ResourceHandler) clientFor(request *backend.CallResourceRequest, streaming bool) (generated.ClientInterface, error) {
	if request != nil && request.PluginContext.AppInstanceSettings != nil {
		var settings appSettings
		if err := json.Unmarshal(request.PluginContext.AppInstanceSettings.JSONData, &settings); err != nil {
			return nil, err
		}
		endpoint := strings.TrimSpace(settings.AICoreEndpoint)
		newClient := h.NewClient
		if streaming && h.NewStreamClient != nil {
			newClient = h.NewStreamClient
		}
		if endpoint == "" || newClient == nil {
			return nil, errors.New("aiCoreEndpoint is missing")
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
			return nil, errors.New("aiCoreEndpoint must be an HTTP(S) URL without credentials")
		}
		return newClient(strings.TrimRight(endpoint, "/"))
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
func (h *ResourceHandler) listMessages(ctx context.Context, client generated.ClientInterface, request *backend.CallResourceRequest, sessionID string, identity requestcontext.Context, sender backend.CallResourceResponseSender) error {
	params, err := messagePageParams(request, identity)
	if err != nil {
		return h.sendError(sender, http.StatusBadRequest, "invalid_argument", "Invalid message page parameters", identity.RequestID, false)
	}
	response, err := client.ListSessionMessages(ctx, sessionID, params)
	return h.forward(sender, response, err, identity.RequestID, false)
}
func (h *ResourceHandler) listTasks(ctx context.Context, client generated.ClientInterface, request *backend.CallResourceRequest, sessionID string, identity requestcontext.Context, sender backend.CallResourceResponseSender) error {
	params, err := taskPageParams(request, identity)
	if err != nil {
		return h.sendError(sender, http.StatusBadRequest, "invalid_argument", "Invalid task page parameters", identity.RequestID, false)
	}
	response, err := client.ListSessionTasks(ctx, sessionID, params)
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
func (h *ResourceHandler) replayEvents(ctx context.Context, client generated.ClientInterface, request *backend.CallResourceRequest, taskID string, identity requestcontext.Context, sender backend.CallResourceResponseSender) error {
	params, err := replayParams(request, identity)
	if err != nil {
		return h.sendError(sender, http.StatusBadRequest, "invalid_argument", "Invalid event replay parameters", identity.RequestID, false)
	}
	response, err := client.ReplayTaskEvents(ctx, taskID, params)
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
func messagePageParams(request *backend.CallResourceRequest, identity requestcontext.Context) (*generated.ListSessionMessagesParams, error) {
	query, err := resourceQuery(request)
	if err != nil {
		return nil, err
	}
	params := &generated.ListSessionMessagesParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), XRequestID: identity.RequestID, XTraceID: identity.TraceID}
	if err := setPageSize(query, "pageSize", 100, func(value int) { params.PageSize = &value }); err != nil {
		return nil, err
	}
	if err := setPageToken(query, func(value string) { params.PageToken = &value }); err != nil {
		return nil, err
	}
	return params, nil
}
func taskPageParams(request *backend.CallResourceRequest, identity requestcontext.Context) (*generated.ListSessionTasksParams, error) {
	query, err := resourceQuery(request)
	if err != nil {
		return nil, err
	}
	params := &generated.ListSessionTasksParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), XRequestID: identity.RequestID, XTraceID: identity.TraceID}
	if err := setPageSize(query, "pageSize", 50, func(value int) { params.PageSize = &value }); err != nil {
		return nil, err
	}
	if err := setPageToken(query, func(value string) { params.PageToken = &value }); err != nil {
		return nil, err
	}
	return params, nil
}
func taskParams(identity requestcontext.Context, key string) *generated.CreateTaskParams {
	return &generated.CreateTaskParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), IdempotencyKey: key, XRequestID: identity.RequestID, XTraceID: identity.TraceID}
}
func getTaskParams(identity requestcontext.Context) *generated.GetTaskParams {
	return &generated.GetTaskParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), XRequestID: identity.RequestID, XTraceID: identity.TraceID}
}
func eventParams(request *backend.CallResourceRequest, identity requestcontext.Context) (*generated.StreamTaskEventsParams, error) {
	params := &generated.StreamTaskEventsParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), XRequestID: identity.RequestID, XTraceID: identity.TraceID}
	parsed, err := resourceQuery(request)
	if err != nil {
		return nil, err
	}
	if value := parsed.Get("afterSequence"); value != "" {
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
func replayParams(request *backend.CallResourceRequest, identity requestcontext.Context) (*generated.ReplayTaskEventsParams, error) {
	query, err := resourceQuery(request)
	if err != nil {
		return nil, err
	}
	params := &generated.ReplayTaskEventsParams{XMTBTenantID: identity.TenantID, XMTBOrgID: identity.OrgID, XMTBUserID: identity.UserID, XMTBRoles: strings.Join(identity.Roles, ","), XMTBPermissions: strings.Join(identity.Permissions, ","), XRequestID: identity.RequestID, XTraceID: identity.TraceID}
	if err := setNonNegativeInteger(query, "afterSequence", func(value int) { params.AfterSequence = &value }); err != nil {
		return nil, err
	}
	if err := setPageSize(query, "pageSize", 200, func(value int) { params.PageSize = &value }); err != nil {
		return nil, err
	}
	if err := setPageToken(query, func(value string) { params.PageToken = &value }); err != nil {
		return nil, err
	}
	if params.AfterSequence != nil && params.PageToken != nil {
		return nil, errors.New("afterSequence and pageToken are mutually exclusive")
	}
	return params, nil
}
func resourceQuery(request *backend.CallResourceRequest) (url.Values, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		return nil, err
	}
	return parsed.Query(), nil
}
func setPageSize(query url.Values, key string, maximum int, set func(int)) error {
	values, exists := query[key]
	if !exists {
		return nil
	}
	if len(values) != 1 {
		return errors.New("invalid page size")
	}
	value, err := strconv.Atoi(values[0])
	if err != nil || value < 1 || value > maximum {
		return errors.New("invalid page size")
	}
	set(value)
	return nil
}
func setNonNegativeInteger(query url.Values, key string, set func(int)) error {
	values, exists := query[key]
	if !exists {
		return nil
	}
	if len(values) != 1 || values[0] == "" {
		return errors.New("invalid query parameter")
	}
	value, err := strconv.Atoi(values[0])
	if err != nil || value < 0 {
		return errors.New("invalid query parameter")
	}
	set(value)
	return nil
}
func setPageToken(query url.Values, set func(string)) error {
	values, exists := query["pageToken"]
	if !exists {
		return nil
	}
	if len(values) != 1 || strings.TrimSpace(values[0]) == "" {
		return errors.New("invalid page token")
	}
	set(values[0])
	return nil
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
