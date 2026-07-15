// Package httpapi adapts the generated OpenAPI server surface to application commands.
package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	generated "mini-torchbearing.local/services/ai-core/internal/adapters/inbound/http/generated"
	"mini-torchbearing.local/services/ai-core/internal/application/approvals"
	"mini-torchbearing.local/services/ai-core/internal/application/commands"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/remediation"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

type API struct {
	Commands     *commands.Service
	Approvals    ApprovalService
	Incidents    AlertIngestor
	AlertIngress AlertIngressConfig
	Store        repositories.ApplicationStore
	Notifier     events.Notifier
	Readiness    func(context.Context) error
}

type ApprovalService interface {
	Get(context.Context, requestcontext.Context, string) (remediation.Approval, error)
	Decide(context.Context, requestcontext.Context, approvals.DecisionInput) (remediation.Approval, error)
}

var _ generated.ServerInterface = (*API)(nil)

func NewHandler(api *API) http.Handler {
	return generated.HandlerWithOptions(api, generated.StdHTTPServerOptions{ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
		writeError(w, r.Header.Get("X-Request-ID"), common.NewError(common.InvalidArgument, err.Error(), false))
	}})
}

func (a *API) Healthz(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func (a *API) ListIncidents(w http.ResponseWriter, _ *http.Request, params generated.ListIncidentsParams) {
	writeError(w, params.XRequestID, common.NewError(common.NotImplemented, "Incident listing is not implemented", false))
}

func (a *API) GetTaskApproval(w http.ResponseWriter, r *http.Request, taskID generated.TaskId, params generated.GetTaskApprovalParams) {
	if a.Approvals == nil {
		writeError(w, params.XRequestID, common.NewError(common.NotImplemented, "Incident approval is not configured", false))
		return
	}
	result, err := a.Approvals.Get(r.Context(), identity(params.XMTBTenantID, params.XMTBOrgID, params.XMTBUserID, params.XMTBRoles, params.XMTBPermissions, params.XRequestID, params.XTraceID), taskID)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	writeJSON(w, http.StatusOK, approvalResponse(result))
}

func (a *API) DecideTaskApproval(w http.ResponseWriter, r *http.Request, taskID generated.TaskId, params generated.DecideTaskApprovalParams) {
	if a.Approvals == nil {
		writeError(w, params.XRequestID, common.NewError(common.NotImplemented, "Incident approval is not configured", false))
		return
	}
	var body generated.DecideTaskApprovalJSONRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	result, err := a.Approvals.Decide(r.Context(), identity(params.XMTBTenantID, params.XMTBOrgID, params.XMTBUserID, params.XMTBRoles, params.XMTBPermissions, params.XRequestID, params.XTraceID), approvals.DecisionInput{TaskID: taskID, Decision: string(body.Decision), Reason: body.Reason, IntentDigest: body.IntentDigest, IdempotencyKey: params.IdempotencyKey, ExpectedTaskVersion: int64(body.ExpectedTaskVersion), ExpectedApprovalVersion: int64(body.ExpectedApprovalVersion)})
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	writeJSON(w, http.StatusAccepted, approvalResponse(result))
}

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

func (a *API) ListSessions(w http.ResponseWriter, r *http.Request, params generated.ListSessionsParams) {
	page, err := sessionPageRequest(params.PageSize, params.PageToken, params.XMTBTenantID, params.XMTBUserID)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	result, err := a.Store.Sessions().ListPageByOwner(r.Context(), params.XMTBTenantID, params.XMTBUserID, page)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	next, err := encodeSessionListToken(params.XMTBTenantID, params.XMTBUserID, result.NextAfter)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	items := make([]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, sessionResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextPageToken": next})
}

func (a *API) GetSession(w http.ResponseWriter, r *http.Request, sessionID generated.SessionId, params generated.GetSessionParams) {
	result, err := a.Store.Sessions().GetOwned(r.Context(), params.XMTBTenantID, params.XMTBUserID, sessionID)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse(result))
}

func (a *API) ListSessionMessages(w http.ResponseWriter, r *http.Request, sessionID generated.SessionId, params generated.ListSessionMessagesParams) {
	if _, err := a.Store.Sessions().GetOwned(r.Context(), params.XMTBTenantID, params.XMTBUserID, sessionID); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	page, err := messagePageRequest(params.PageSize, params.PageToken, sessionID)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	result, err := a.Store.Messages().ListPageBySession(r.Context(), params.XMTBTenantID, sessionID, page)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	next, err := encodeListToken("messages", sessionID, result.NextAfter)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	items := make([]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, messageResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextPageToken": next})
}

func (a *API) ListSessionTasks(w http.ResponseWriter, r *http.Request, sessionID generated.SessionId, params generated.ListSessionTasksParams) {
	if _, err := a.Store.Sessions().GetOwned(r.Context(), params.XMTBTenantID, params.XMTBUserID, sessionID); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	page, err := taskPageRequest(params.PageSize, params.PageToken, sessionID)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	result, err := a.Store.Tasks().ListPageBySession(r.Context(), params.XMTBTenantID, sessionID, page)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	next, err := encodeListToken("tasks", sessionID, result.NextAfter)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	items := make([]any, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, taskResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextPageToken": next})
}

func (a *API) CreateTask(w http.ResponseWriter, r *http.Request, params generated.CreateTaskParams) {
	var body generated.CreateTaskJSONRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	datasourceUID := string(body.AnalysisContext.DatasourceUid)
	if strings.TrimSpace(datasourceUID) == "" {
		writeError(w, params.XRequestID, common.NewError(common.InvalidArgument, "analysisContext.datasourceUid must be a string", false))
		return
	}
	timeRange, err := parseTimeRange(body.AnalysisContext.TimeRange)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	stepSeconds, err := parseResolution(body.AnalysisContext.Resolution)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	result, err := a.Commands.CreateTask(r.Context(), identity(params.XMTBTenantID, params.XMTBOrgID, params.XMTBUserID, params.XMTBRoles, params.XMTBPermissions, params.XRequestID, params.XTraceID), commands.CreateTaskInput{SessionID: body.SessionId, Message: body.Message, DatasourceUID: datasourceUID, TimeRange: timeRange, StepSeconds: stepSeconds, IdempotencyKey: params.IdempotencyKey, RequestHash: canonicalTaskRequestHash(params.XMTBTenantID, body)})
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	writeJSON(w, http.StatusAccepted, taskResponse(result))
}

func canonicalTaskRequestHash(tenantID string, body generated.CreateTaskJSONRequestBody) string {
	encoded, _ := json.Marshal(struct {
		TenantID string                              `json:"tenantId"`
		Body     generated.CreateTaskJSONRequestBody `json:"body"`
	}{TenantID: tenantID, Body: body})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (a *API) GetTask(w http.ResponseWriter, r *http.Request, taskID generated.TaskId, params generated.GetTaskParams) {
	result, err := a.getOwnedTask(r.Context(), params.XMTBTenantID, params.XMTBUserID, taskID)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	writeJSON(w, http.StatusOK, taskResponse(result))
}

func (a *API) StreamTaskEvents(w http.ResponseWriter, r *http.Request, taskID generated.TaskId, params generated.StreamTaskEventsParams) {
	if _, err := a.getOwnedTask(r.Context(), params.XMTBTenantID, params.XMTBUserID, taskID); err != nil {
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
		current, err := a.Store.Tasks().Get(r.Context(), params.XMTBTenantID, taskID)
		if err != nil {
			return
		}
		if (current.Status == task.StatusCompleted || current.Status == task.StatusFailed) && after >= current.LatestSequence {
			latest, replayErr := a.Store.TaskEvents().Replay(r.Context(), params.XMTBTenantID, taskID, current.LatestSequence-1, 1)
			if replayErr != nil {
				return
			}
			if len(latest) == 1 && (latest[0].Type == task.EventTaskCompleted || latest[0].Type == task.EventTaskFailed) {
				return
			}
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

func (a *API) ReplayTaskEvents(w http.ResponseWriter, r *http.Request, taskID generated.TaskId, params generated.ReplayTaskEventsParams) {
	if _, err := a.getOwnedTask(r.Context(), params.XMTBTenantID, params.XMTBUserID, taskID); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	if params.AfterSequence != nil && params.PageToken != nil {
		writeError(w, params.XRequestID, common.NewError(common.InvalidArgument, "afterSequence and pageToken are mutually exclusive", false))
		return
	}
	pageSize := pageSize(params.PageSize, 200, 200)
	if pageSize == 0 {
		writeError(w, params.XRequestID, common.NewError(common.InvalidArgument, "pageSize is invalid", false))
		return
	}
	after, target, err := replayCursor(params.AfterSequence, params.PageToken, taskID)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	if params.PageToken == nil {
		item, getErr := a.Store.Tasks().Get(r.Context(), params.XMTBTenantID, taskID)
		if getErr != nil {
			writeError(w, params.XRequestID, getErr)
			return
		}
		target = item.LatestSequence
	}
	items, err := a.Store.TaskEvents().ReplayTo(r.Context(), params.XMTBTenantID, taskID, after, target, pageSize)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	next := any(nil)
	if len(items) == pageSize && items[len(items)-1].Sequence < target {
		next, err = encodeReplayToken(taskID, target, items[len(items)-1].Sequence)
		if err != nil {
			writeError(w, params.XRequestID, err)
			return
		}
	}
	events := make([]any, 0, len(items))
	for _, item := range items {
		events = append(events, taskEventResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events, "targetSequence": target, "nextPageToken": next})
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

func parseTimeRange(value *generated.CreateTaskRequestSchema_AnalysisContext_TimeRange) (commands.RequestedTimeRange, error) {
	if value == nil {
		return commands.RequestedTimeRange{}, nil
	}
	if absolute, err := value.AsCreateTaskRequestSchemaAnalysisContextTimeRange0(); err == nil && !absolute.From.IsZero() && !absolute.To.IsZero() {
		resolved, rangeErr := common.NewAbsoluteTimeRange(absolute.From, absolute.To)
		if rangeErr != nil {
			return commands.RequestedTimeRange{}, rangeErr
		}
		return commands.RequestedTimeRange{Absolute: &resolved}, nil
	}
	if relative, err := value.AsCreateTaskRequestSchemaAnalysisContextTimeRange1(); err == nil && relative.RelativeDuration != "" {
		duration, parseErr := time.ParseDuration(relative.RelativeDuration)
		if parseErr != nil || duration <= 0 {
			return commands.RequestedTimeRange{}, common.NewError(common.InvalidArgument, "relativeDuration must be a positive Go duration", false)
		}
		return commands.RequestedTimeRange{RelativeDuration: duration}, nil
	}
	return commands.RequestedTimeRange{}, common.NewError(common.InvalidArgument, "timeRange must be absolute or relative", false)
}

func parseResolution(value *generated.CreateTaskRequestSchema_AnalysisContext_Resolution) (*int, error) {
	if value == nil {
		return nil, nil
	}
	if explicit, err := value.AsCreateTaskRequestSchemaAnalysisContextResolution1(); err == nil && int(explicit.StepSeconds) > 0 {
		step := int(explicit.StepSeconds)
		return &step, nil
	}
	if automatic, err := value.AsCreateTaskRequestSchemaAnalysisContextResolution0(); err == nil && fmt.Sprint(automatic.Mode) == "auto" {
		return nil, nil
	}
	return nil, common.NewError(common.InvalidArgument, "resolution must be auto or a supported stepSeconds value", false)
}

type listPageToken struct {
	Version   int    `json:"version"`
	Resource  string `json:"resource"`
	SessionID string `json:"sessionId"`
	CreatedAt string `json:"createdAt"`
	ID        string `json:"id"`
}

type sessionListPageToken struct {
	Version   int    `json:"version"`
	Resource  string `json:"resource"`
	TenantID  string `json:"tenantId"`
	UserID    string `json:"userId"`
	UpdatedAt string `json:"updatedAt"`
	ID        string `json:"id"`
}

type taskReplayToken struct {
	Version        int    `json:"version"`
	Resource       string `json:"resource"`
	TaskID         string `json:"taskId"`
	TargetSequence int64  `json:"targetSequence"`
	AfterSequence  int64  `json:"afterSequence"`
}

func messagePageRequest(size *int, token *string, sessionID string) (repositories.PageRequest, error) {
	return listPageRequest(size, token, sessionID, "messages", 50, 100)
}

func taskPageRequest(size *int, token *string, sessionID string) (repositories.PageRequest, error) {
	return listPageRequest(size, token, sessionID, "tasks", 20, 50)
}

func sessionPageRequest(size *int, encoded *string, tenantID, userID string) (repositories.SessionListRequest, error) {
	result := repositories.SessionListRequest{Limit: pageSize(size, 20, 50)}
	if result.Limit == 0 || tenantID == "" || userID == "" {
		return repositories.SessionListRequest{}, common.NewError(common.InvalidArgument, "session page request is invalid", false)
	}
	if encoded == nil {
		return result, nil
	}
	var token sessionListPageToken
	if err := decodePageToken(*encoded, &token); err != nil || token.Version != 1 || token.Resource != "sessions" || token.TenantID != tenantID || token.UserID != userID || token.ID == "" {
		return repositories.SessionListRequest{}, common.NewError(common.InvalidArgument, "pageToken is invalid", false)
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, token.UpdatedAt)
	if err != nil {
		return repositories.SessionListRequest{}, common.NewError(common.InvalidArgument, "pageToken is invalid", false)
	}
	updatedAt = updatedAt.UTC()
	result.BeforeUpdatedAt, result.BeforeID = &updatedAt, token.ID
	return result, nil
}

func listPageRequest(size *int, encoded *string, sessionID, resource string, defaultSize, maximum int) (repositories.PageRequest, error) {
	result := repositories.PageRequest{Limit: pageSize(size, defaultSize, maximum)}
	if result.Limit == 0 {
		return repositories.PageRequest{}, common.NewError(common.InvalidArgument, "pageSize is invalid", false)
	}
	if encoded == nil {
		return result, nil
	}
	var token listPageToken
	if err := decodePageToken(*encoded, &token); err != nil || token.Version != 1 || token.Resource != resource || token.SessionID != sessionID || token.ID == "" {
		return repositories.PageRequest{}, common.NewError(common.InvalidArgument, "pageToken is invalid", false)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, token.CreatedAt)
	if err != nil {
		return repositories.PageRequest{}, common.NewError(common.InvalidArgument, "pageToken is invalid", false)
	}
	createdAt = createdAt.UTC()
	result.CreatedAt, result.ID = &createdAt, token.ID
	return result, nil
}

func replayCursor(afterSequence *int, encoded *string, taskID string) (int64, int64, error) {
	if encoded == nil {
		if afterSequence == nil {
			return 0, 0, nil
		}
		return int64(*afterSequence), 0, nil
	}
	var token taskReplayToken
	if err := decodePageToken(*encoded, &token); err != nil || token.Version != 1 || token.Resource != "task_events_replay" || token.TaskID != taskID || token.AfterSequence < 0 || token.TargetSequence < token.AfterSequence {
		return 0, 0, common.NewError(common.InvalidArgument, "pageToken is invalid", false)
	}
	return token.AfterSequence, token.TargetSequence, nil
}

func encodeListToken(resource, sessionID string, cursor *repositories.PageCursor) (any, error) {
	if cursor == nil {
		return nil, nil
	}
	return encodePageToken(listPageToken{Version: 1, Resource: resource, SessionID: sessionID, CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano), ID: cursor.ID})
}

func encodeSessionListToken(tenantID, userID string, cursor *repositories.SessionListCursor) (any, error) {
	if cursor == nil {
		return nil, nil
	}
	return encodePageToken(sessionListPageToken{Version: 1, Resource: "sessions", TenantID: tenantID, UserID: userID, UpdatedAt: cursor.UpdatedAt.UTC().Format(time.RFC3339Nano), ID: cursor.ID})
}

func encodeReplayToken(taskID string, target, after int64) (string, error) {
	return encodePageToken(taskReplayToken{Version: 1, Resource: "task_events_replay", TaskID: taskID, TargetSequence: target, AfterSequence: after})
}

func encodePageToken(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", common.NewError(common.InternalError, "cannot encode page token", false)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodePageToken(value string, destination any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(decoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	return nil
}

func pageSize(value *int, defaultSize, maximum int) int {
	if value == nil {
		return defaultSize
	}
	if *value < 1 || *value > maximum {
		return 0
	}
	return *value
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
	return map[string]any{"id": value.ID, "tenantId": value.TenantID, "orgId": value.OrgID, "kind": value.Kind, "title": value.Title, "status": value.Status, "createdBy": value.CreatedBy, "createdAt": value.CreatedAt, "updatedAt": value.UpdatedAt, "version": value.Version}
}
func taskResponse(value task.AnalysisTask) any {
	var failure any
	if value.Error != nil {
		failure = map[string]any{"code": value.Error.Code, "message": value.Error.Message, "retryable": value.Error.Retryable, "requestId": ""}
	}
	response := map[string]any{"id": value.ID, "kind": value.Kind, "sessionId": value.SessionID, "status": value.Status, "inputMessageId": value.InputMessageID, "latestSequence": value.LatestSequence, "error": failure, "createdAt": value.CreatedAt, "startedAt": value.StartedAt, "completedAt": value.CompletedAt, "updatedAt": value.UpdatedAt, "version": value.Version}
	if value.Kind == task.KindMetricAnalysis {
		response["datasourceUid"] = value.DatasourceUID
		response["timeRange"] = map[string]any{"from": value.TimeRange.From, "to": value.TimeRange.To}
		response["queryPlan"] = map[string]any{"views": value.QueryPlan.Views, "stepSeconds": value.QueryPlan.StepSeconds, "cpuRateWindowSeconds": value.QueryPlan.CPURateWindowSeconds}
	} else {
		response["incidentPlan"] = value.IncidentPlan
	}
	return response
}
func messageResponse(value session.Message) any {
	return map[string]any{"id": value.ID, "sessionId": value.SessionID, "taskId": value.TaskID, "role": value.Role, "content": value.Content, "createdAt": value.CreatedAt}
}
func approvalResponse(value remediation.Approval) any {
	return map[string]any{"id": value.ID, "taskId": value.TaskID, "status": value.Status, "intentDigest": value.IntentDigest, "requestedAt": value.RequestedAt, "expiresAt": value.ExpiresAt, "decidedAt": value.DecidedAt, "decidedBy": value.DecidedBy, "decisionReason": value.DecisionReason, "version": value.Version}
}
func taskEventResponse(event task.TaskEvent) any {
	return map[string]any{"eventId": event.EventID, "taskId": event.TaskID, "sessionId": event.SessionID, "sequence": event.Sequence, "type": event.Type, "timestamp": event.Timestamp, "payload": json.RawMessage(event.Payload)}
}

func (a *API) getOwnedTask(ctx context.Context, tenantID, userID, taskID string) (task.AnalysisTask, error) {
	result, err := a.Store.Tasks().Get(ctx, tenantID, taskID)
	if err != nil {
		return task.AnalysisTask{}, err
	}
	if _, err := a.Store.Sessions().GetOwned(ctx, tenantID, userID, result.SessionID); err != nil {
		return task.AnalysisTask{}, err
	}
	return result, nil
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
	status := map[common.ErrorCode]int{common.InvalidArgument: http.StatusBadRequest, common.SchemaValidationFailed: http.StatusBadRequest, common.Unauthenticated: http.StatusUnauthorized, common.PermissionDenied: http.StatusForbidden, common.ResourceNotFound: http.StatusNotFound, common.ResourceConflict: http.StatusConflict, common.InvalidStateTransition: http.StatusConflict, common.IdempotencyConflict: http.StatusConflict, common.TargetPreconditionFailed: http.StatusConflict, common.ApprovalRequired: http.StatusConflict, common.ApprovalExpired: http.StatusConflict, common.DependencyUnavailable: http.StatusServiceUnavailable, common.ToolTimeout: http.StatusGatewayTimeout, common.NotImplemented: http.StatusNotImplemented}[domainErr.Code]
	if status == 0 {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": domainErr.Code, "message": domainErr.Message, "retryable": domainErr.Retryable, "requestId": requestID, "details": domainErr.Details}})
}
