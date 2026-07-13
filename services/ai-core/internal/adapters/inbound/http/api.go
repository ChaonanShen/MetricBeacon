// Package httpapi adapts the generated OpenAPI server surface to application commands.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	generated "mini-torchbearing.local/services/ai-core/internal/adapters/inbound/http/generated"
	"mini-torchbearing.local/services/ai-core/internal/application/commands"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

type API struct {
	Commands  *commands.Service
	Store     repositories.ApplicationStore
	Notifier  events.Notifier
	Readiness func(context.Context) error
}

var _ generated.ServerInterface = (*API)(nil)

func NewHandler(api *API) http.Handler {
	return generated.HandlerWithOptions(api, generated.StdHTTPServerOptions{ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
		writeError(w, r.Header.Get("X-Request-ID"), common.NewError(common.InvalidArgument, err.Error(), false))
	}})
}

func (a *API) Healthz(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
func (a *API) Readyz(w http.ResponseWriter, r *http.Request) {
	if a == nil || a.Store == nil {
		writeError(w, "", common.NewError(common.InternalError, "AI Core is not configured", true))
		return
	}
	if err := a.Store.Health(r.Context()); err != nil {
		writeError(w, "", err)
		return
	}
	if a.Readiness != nil {
		if err := a.Readiness(r.Context()); err != nil {
			writeError(w, "", err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (a *API) CreateSession(w http.ResponseWriter, r *http.Request, params generated.CreateSessionParams) {
	var body generated.CreateSessionJSONRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	title := "Node exporter overview"
	if body.Title != nil {
		title = *body.Title
	}
	result, err := a.Commands.CreateSession(r.Context(), identity(params.XMTBTenantID, params.XMTBOrgID, params.XMTBUserID, params.XMTBRoles, params.XMTBPermissions, params.XRequestID, params.XTraceID), commands.CreateSessionInput{Title: title, IdempotencyKey: params.XRequestID})
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	writeJSON(w, http.StatusCreated, sessionResponse(result))
}

func (a *API) GetSession(w http.ResponseWriter, r *http.Request, sessionID generated.SessionId, params generated.GetSessionParams) {
	result, err := a.Store.Sessions().Get(r.Context(), params.XMTBTenantID, sessionID)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse(result))
}

func (a *API) CreateTask(w http.ResponseWriter, r *http.Request, params generated.CreateTaskParams) {
	var body generated.CreateTaskJSONRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	datasourceUID, ok := body.AnalysisContext.DatasourceUid.(string)
	if !ok || strings.TrimSpace(datasourceUID) == "" {
		writeError(w, params.XRequestID, common.NewError(common.InvalidArgument, "analysisContext.datasourceUid must be a string", false))
		return
	}
	timeRange, err := parseTimeRange(body.AnalysisContext.TimeRange)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	result, err := a.Commands.CreateTask(r.Context(), identity(params.XMTBTenantID, params.XMTBOrgID, params.XMTBUserID, params.XMTBRoles, params.XMTBPermissions, params.XRequestID, params.XTraceID), commands.CreateTaskInput{SessionID: body.SessionId, Message: body.Message, DatasourceUID: datasourceUID, TimeRange: timeRange, IdempotencyKey: params.IdempotencyKey})
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	writeJSON(w, http.StatusAccepted, taskResponse(result))
}

func (a *API) GetTask(w http.ResponseWriter, r *http.Request, taskID generated.TaskId, params generated.GetTaskParams) {
	result, err := a.Store.Tasks().Get(r.Context(), params.XMTBTenantID, taskID)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	writeJSON(w, http.StatusOK, taskResponse(result))
}

func (a *API) StreamTaskEvents(w http.ResponseWriter, r *http.Request, taskID generated.TaskId, params generated.StreamTaskEventsParams) {
	if _, err := a.Store.Tasks().Get(r.Context(), params.XMTBTenantID, taskID); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, params.XRequestID, common.NewError(common.InternalError, "response streaming is not supported", true))
		return
	}
	after := int64(0)
	if params.AfterSequence != nil {
		after = int64(*params.AfterSequence)
	}
	if params.LastEventID != nil && int64(*params.LastEventID) > after {
		after = int64(*params.LastEventID)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	var notifications <-chan struct{}
	if a.Notifier != nil {
		notifications, _ = a.Notifier.Subscribe(r.Context(), params.XMTBTenantID, taskID)
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		events, err := a.Store.TaskEvents().Replay(r.Context(), params.XMTBTenantID, taskID, after, 200)
		if err != nil {
			return
		}
		for _, event := range events {
			if err := writeEvent(w, event); err != nil {
				return
			}
			after = event.Sequence
		}
		if len(events) > 0 {
			flusher.Flush()
			continue
		}
		select {
		case <-r.Context().Done():
			return
		case <-notifications:
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func decodeJSON(r *http.Request, value any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return common.NewError(common.InvalidArgument, "invalid JSON request body", false)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return common.NewError(common.InvalidArgument, "request body must contain one JSON value", false)
	}
	return nil
}

func parseTimeRange(value *generated.CreateTaskRequestSchema_AnalysisContext_TimeRange) (common.AbsoluteTimeRange, error) {
	now := time.Now().UTC()
	if value == nil {
		return common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	}
	if absolute, err := value.AsCreateTaskRequestSchemaAnalysisContextTimeRange0(); err == nil && !absolute.From.IsZero() && !absolute.To.IsZero() {
		return common.NewAbsoluteTimeRange(absolute.From, absolute.To)
	}
	if relative, err := value.AsCreateTaskRequestSchemaAnalysisContextTimeRange1(); err == nil && relative.RelativeDuration != "" {
		duration, parseErr := time.ParseDuration(relative.RelativeDuration)
		if parseErr != nil || duration <= 0 {
			return common.AbsoluteTimeRange{}, common.NewError(common.InvalidArgument, "relativeDuration must be a positive Go duration", false)
		}
		return common.NewAbsoluteTimeRange(now.Add(-duration), now)
	}
	return common.AbsoluteTimeRange{}, common.NewError(common.InvalidArgument, "timeRange must be absolute or relative", false)
}

func identity(tenant, org, user, roles, permissions, requestID, traceID string) requestcontext.Context {
	return requestcontext.Context{TenantID: tenant, OrgID: org, UserID: user, Roles: split(roles), Permissions: split(permissions), RequestID: requestID, TraceID: traceID}
}
func split(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func sessionResponse(value session.AnalysisSession) any {
	return map[string]any{"id": value.ID, "tenantId": value.TenantID, "title": value.Title, "status": value.Status, "createdBy": value.CreatedBy, "createdAt": value.CreatedAt, "updatedAt": value.UpdatedAt, "version": value.Version}
}
func taskResponse(value task.AnalysisTask) any {
	var failure any
	if value.Error != nil {
		failure = map[string]any{"code": value.Error.Code, "message": value.Error.Message, "retryable": value.Error.Retryable, "requestId": ""}
	}
	return map[string]any{"id": value.ID, "sessionId": value.SessionID, "status": value.Status, "inputMessageId": value.InputMessageID, "datasourceUid": value.DatasourceUID, "timeRange": map[string]any{"from": value.TimeRange.From, "to": value.TimeRange.To}, "latestSequence": value.LatestSequence, "error": failure, "createdAt": value.CreatedAt, "startedAt": value.StartedAt, "completedAt": value.CompletedAt, "updatedAt": value.UpdatedAt, "version": value.Version}
}

func writeEvent(w http.ResponseWriter, event task.TaskEvent) error {
	payload := map[string]any{"eventId": event.EventID, "taskId": event.TaskID, "sessionId": event.SessionID, "sequence": event.Sequence, "type": event.Type, "timestamp": event.Timestamp, "payload": json.RawMessage(event.Payload)}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Type, encoded)
	return err
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, requestID string, err error) {
	var domainErr *common.DomainError
	if !errors.As(err, &domainErr) {
		domainErr = common.NewError(common.InternalError, "internal server error", true)
	}
	status := map[common.ErrorCode]int{common.InvalidArgument: http.StatusBadRequest, common.PermissionDenied: http.StatusForbidden, common.ResourceNotFound: http.StatusNotFound, common.ResourceConflict: http.StatusConflict, common.IdempotencyConflict: http.StatusConflict, common.DependencyUnavailable: http.StatusServiceUnavailable, common.ToolTimeout: http.StatusGatewayTimeout}[domainErr.Code]
	if status == 0 {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": domainErr.Code, "message": domainErr.Message, "retryable": domainErr.Retryable, "requestId": requestID, "details": domainErr.Details}})
}
