package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	clockadapter "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/clocks"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/events/inmemory"
	idadapter "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/ids"
	storage "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/storage/sqlite"
	"mini-torchbearing.local/services/ai-core/internal/application/commands"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/application/workflows"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

func TestGeneratedHTTPHandlersCreateAndStreamTask(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "ai-core.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	notifier := inmemory.New()
	clock, generator := clockadapter.NewSystem(), idadapter.New()
	workflow := workflows.RunAnalysisWorkflow{Store: store, Notifier: notifier, Runtime: httpRuntime{}, IDs: generator, Clock: clock}
	planner := &httpPlanner{}
	api := &API{Store: store, Notifier: notifier, Commands: commands.New(store, notifier, workflow, generator, clock, planner)}
	server := httptest.NewServer(NewHandler(api))
	defer server.Close()

	sessionResponse := request(t, http.MethodPost, server.URL+"/v1/sessions", `{"title":"Overview"}`, "request-session", "")
	if sessionResponse.StatusCode != http.StatusCreated {
		t.Fatalf("session response: %d", sessionResponse.StatusCode)
	}
	var sessionBody struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(sessionResponse.Body).Decode(&sessionBody); err != nil {
		t.Fatal(err)
	}
	sessionResponse.Body.Close()
	if sessionBody.ID == "" {
		t.Fatal("session id is missing")
	}
	emptySessionsResponse := request(t, http.MethodGet, server.URL+"/v1/sessions?pageSize=20", "", "request-empty-sessions", "")
	var emptySessionsPage struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(emptySessionsResponse.Body).Decode(&emptySessionsPage); err != nil {
		t.Fatal(err)
	}
	emptySessionsResponse.Body.Close()
	if emptySessionsResponse.StatusCode != http.StatusOK || len(emptySessionsPage.Items) != 0 {
		t.Fatalf("empty Session page = %#v status=%d", emptySessionsPage, emptySessionsResponse.StatusCode)
	}

	body := `{"sessionId":"` + sessionBody.ID + `","message":"show node exporter","analysisContext":{"datasourceUid":"prometheus-main","timeRange":{"relativeDuration":"30m"}}}`
	taskResponse := request(t, http.MethodPost, server.URL+"/v1/tasks", body, "request-task", "task-key")
	if taskResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("task response: %d", taskResponse.StatusCode)
	}
	var taskBody struct {
		ID        string `json:"id"`
		QueryPlan struct {
			Views                []string `json:"views"`
			StepSeconds          int      `json:"stepSeconds"`
			CPURateWindowSeconds int      `json:"cpuRateWindowSeconds"`
		} `json:"queryPlan"`
	}
	if err := json.NewDecoder(taskResponse.Body).Decode(&taskBody); err != nil {
		t.Fatal(err)
	}
	taskResponse.Body.Close()
	if taskBody.ID == "" || len(taskBody.QueryPlan.Views) != 1 || taskBody.QueryPlan.Views[0] != "cpu" || taskBody.QueryPlan.StepSeconds != 10 || taskBody.QueryPlan.CPURateWindowSeconds != 60 {
		t.Fatalf("task id or resolved query plan is missing: %#v", taskBody)
	}
	touchedSessionResponse := request(t, http.MethodGet, server.URL+"/v1/sessions/"+sessionBody.ID, "", "request-touched-session", "")
	var touchedSession struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	if err := json.NewDecoder(touchedSessionResponse.Body).Decode(&touchedSession); err != nil {
		t.Fatal(err)
	}
	touchedSessionResponse.Body.Close()
	if touchedSessionResponse.StatusCode != http.StatusOK || touchedSession.ID != sessionBody.ID || touchedSession.Version != 2 {
		t.Fatalf("touched Session = %#v status=%d", touchedSession, touchedSessionResponse.StatusCode)
	}
	sessionsResponse := request(t, http.MethodGet, server.URL+"/v1/sessions?pageSize=20", "", "request-sessions", "")
	var sessionsPage struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		NextPageToken *string `json:"nextPageToken"`
	}
	if err := json.NewDecoder(sessionsResponse.Body).Decode(&sessionsPage); err != nil {
		t.Fatal(err)
	}
	sessionsResponse.Body.Close()
	if sessionsResponse.StatusCode != http.StatusOK || len(sessionsPage.Items) != 1 || sessionsPage.Items[0].ID != sessionBody.ID || sessionsPage.NextPageToken != nil {
		t.Fatalf("Session page = %#v status=%d", sessionsPage, sessionsResponse.StatusCode)
	}

	retryResponse := request(t, http.MethodPost, server.URL+"/v1/tasks", body, "request-task-retry", "task-key")
	if retryResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("same-body retry response: %d", retryResponse.StatusCode)
	}
	var retryBody struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(retryResponse.Body).Decode(&retryBody); err != nil {
		t.Fatal(err)
	}
	retryResponse.Body.Close()
	if retryBody.ID != taskBody.ID {
		t.Fatalf("retry task id = %q, want %q", retryBody.ID, taskBody.ID)
	}
	if planner.calls != 1 {
		t.Fatalf("completed retry called planner %d times, want once", planner.calls)
	}
	afterRetry, err := store.Sessions().Get(ctx, "org:1", sessionBody.ID)
	if err != nil || afterRetry.Version != 2 {
		t.Fatalf("idempotent retry touched Session: %#v, %v", afterRetry, err)
	}

	conflictBody := strings.Replace(body, "show node exporter", "different request", 1)
	conflictResponse := request(t, http.MethodPost, server.URL+"/v1/tasks", conflictBody, "request-task-conflict", "task-key")
	defer conflictResponse.Body.Close()
	if conflictResponse.StatusCode != http.StatusConflict {
		t.Fatalf("different-body retry response: %d", conflictResponse.StatusCode)
	}
	planner.failure = common.NewError(common.DependencyUnavailable, "planner unavailable", true)
	failedResponse := request(t, http.MethodPost, server.URL+"/v1/tasks", body, "request-task-planner-failed", "planner-failed-key")
	failedResponse.Body.Close()
	if failedResponse.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("planner failure response: %d", failedResponse.StatusCode)
	}
	afterPlannerFailure, err := store.Sessions().Get(ctx, "org:1", sessionBody.ID)
	if err != nil || afterPlannerFailure.Version != 2 {
		t.Fatalf("planner failure touched Session: %#v, %v", afterPlannerFailure, err)
	}
	_, err = store.Idempotency().GetResult(ctx, repositories.IdempotencyKey{TenantID: "org:1", Scope: "create_task", Key: "planner-failed-key"})
	if !hasHTTPTestCode(err, common.ResourceNotFound) {
		t.Fatalf("planner failure persisted idempotency: %v", err)
	}
	planner.failure = nil

	streamContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	streamRequest, _ := http.NewRequestWithContext(streamContext, http.MethodGet, server.URL+"/v1/tasks/"+taskBody.ID+"/events?afterSequence=0", nil)
	addHeaders(streamRequest, "request-events", "")
	response, err := http.DefaultClient.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("stream response: %d", response.StatusCode)
	}
	scanner := bufio.NewScanner(response.Body)
	foundID, foundData := false, false
	for scanner.Scan() {
		line := scanner.Text()
		foundID = foundID || strings.HasPrefix(line, "id: ")
		foundData = foundData || strings.HasPrefix(line, "data: ")
		if foundID && foundData {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !foundID || !foundData {
		t.Fatal("SSE did not replay durable events")
	}
	waitTaskTerminal(t, store, taskBody.ID)

	messagesResponse := request(t, http.MethodGet, server.URL+"/v1/sessions/"+sessionBody.ID+"/messages?pageSize=1", "", "request-messages", "")
	if messagesResponse.StatusCode != http.StatusOK {
		t.Fatalf("message page response: %d", messagesResponse.StatusCode)
	}
	var messagesPage struct {
		Items []struct {
			ID     string `json:"id"`
			TaskID string `json:"taskId"`
		} `json:"items"`
		NextPageToken *string `json:"nextPageToken"`
	}
	if err := json.NewDecoder(messagesResponse.Body).Decode(&messagesPage); err != nil {
		t.Fatal(err)
	}
	messagesResponse.Body.Close()
	if len(messagesPage.Items) != 1 || messagesPage.Items[0].TaskID != taskBody.ID || messagesPage.NextPageToken == nil {
		t.Fatalf("message page: %#v", messagesPage)
	}
	secondMessagesResponse := request(t, http.MethodGet, server.URL+"/v1/sessions/"+sessionBody.ID+"/messages?pageSize=1&pageToken="+*messagesPage.NextPageToken, "", "request-messages-next", "")
	if secondMessagesResponse.StatusCode != http.StatusOK {
		t.Fatalf("second message page response: %d", secondMessagesResponse.StatusCode)
	}
	secondMessagesResponse.Body.Close()

	replayResponse := request(t, http.MethodGet, server.URL+"/v1/tasks/"+taskBody.ID+"/events/replay?pageSize=1", "", "request-replay", "")
	if replayResponse.StatusCode != http.StatusOK {
		t.Fatalf("replay response: %d", replayResponse.StatusCode)
	}
	var replayPage struct {
		Items          []json.RawMessage `json:"items"`
		TargetSequence int64             `json:"targetSequence"`
		NextPageToken  *string           `json:"nextPageToken"`
	}
	if err := json.NewDecoder(replayResponse.Body).Decode(&replayPage); err != nil {
		t.Fatal(err)
	}
	replayResponse.Body.Close()
	if len(replayPage.Items) != 1 || replayPage.TargetSequence < 1 || replayPage.NextPageToken == nil {
		t.Fatalf("replay page: %#v", replayPage)
	}
	invalidTokenResponse := request(t, http.MethodGet, server.URL+"/v1/sessions/"+sessionBody.ID+"/tasks?pageToken="+*messagesPage.NextPageToken, "", "request-invalid-token", "")
	defer invalidTokenResponse.Body.Close()
	if invalidTokenResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("cross-resource page token response: %d", invalidTokenResponse.StatusCode)
	}

	followupBody := `{"sessionId":"` + sessionBody.ID + `","message":"now show memory","analysisContext":{"datasourceUid":"prometheus-main"}}`
	followupResponse := request(t, http.MethodPost, server.URL+"/v1/tasks", followupBody, "request-followup", "followup-key")
	followupResponse.Body.Close()
	if followupResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("follow-up task response: %d", followupResponse.StatusCode)
	}
	lastRequest := planner.requests[len(planner.requests)-1]
	if len(lastRequest.PreviousIntents) != 1 {
		t.Fatalf("planner previous intents = %#v", lastRequest.PreviousIntents)
	}
	previous := lastRequest.PreviousIntents[0]
	if previous.Message != "show node exporter" || len(previous.Views) != 1 || previous.Views[0] != "cpu" || previous.RangeSeconds != 1800 || previous.StepSeconds != 10 {
		t.Fatalf("planner previous intent = %#v", previous)
	}
}

func TestSessionResourcesAreOwnerScoped(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "owner-scoped.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	notifier := inmemory.New()
	clock, generator := clockadapter.NewSystem(), idadapter.New()
	workflow := workflows.RunAnalysisWorkflow{Store: store, Notifier: notifier, Runtime: httpRuntime{}, IDs: generator, Clock: clock}
	api := &API{Store: store, Notifier: notifier, Commands: commands.New(store, notifier, workflow, generator, clock, &httpPlanner{})}
	server := httptest.NewServer(NewHandler(api))
	defer server.Close()

	sessionResponse := requestAsUser(t, http.MethodPost, server.URL+"/v1/sessions", `{"title":"Private"}`, "owner-session", "", "user:1")
	var sessionBody struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(sessionResponse.Body).Decode(&sessionBody); err != nil {
		t.Fatal(err)
	}
	sessionResponse.Body.Close()
	taskBodyJSON := `{"sessionId":"` + sessionBody.ID + `","message":"show cpu","analysisContext":{"datasourceUid":"prometheus-main"}}`
	taskResponse := requestAsUser(t, http.MethodPost, server.URL+"/v1/tasks", taskBodyJSON, "owner-task", "owner-task-key", "user:1")
	var taskBody struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(taskResponse.Body).Decode(&taskBody); err != nil {
		t.Fatal(err)
	}
	taskResponse.Body.Close()
	if sessionBody.ID == "" || taskBody.ID == "" {
		t.Fatalf("missing owner resources: session=%q task=%q", sessionBody.ID, taskBody.ID)
	}

	foreignList := requestAsUser(t, http.MethodGet, server.URL+"/v1/sessions", "", "foreign-list", "", "user:2")
	var foreignPage struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(foreignList.Body).Decode(&foreignPage); err != nil {
		t.Fatal(err)
	}
	foreignList.Body.Close()
	if foreignList.StatusCode != http.StatusOK || len(foreignPage.Items) != 0 {
		t.Fatalf("foreign Session page = %#v status=%d", foreignPage, foreignList.StatusCode)
	}

	for _, target := range []string{
		"/v1/sessions/" + sessionBody.ID,
		"/v1/sessions/" + sessionBody.ID + "/messages",
		"/v1/sessions/" + sessionBody.ID + "/tasks",
		"/v1/tasks/" + taskBody.ID,
		"/v1/tasks/" + taskBody.ID + "/events/replay?afterSequence=0",
		"/v1/tasks/" + taskBody.ID + "/events?afterSequence=0",
	} {
		response := requestAsUser(t, http.MethodGet, server.URL+target, "", "foreign-read", "", "user:2")
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("foreign GET %s status=%d, want 404", target, response.StatusCode)
		}
	}
	foreignTask := requestAsUser(t, http.MethodPost, server.URL+"/v1/tasks", taskBodyJSON, "foreign-task", "foreign-task-key", "user:2")
	foreignTask.Body.Close()
	if foreignTask.StatusCode != http.StatusNotFound {
		t.Fatalf("foreign Task status=%d, want 404", foreignTask.StatusCode)
	}

	token, err := encodeSessionListToken("org:1", "user:1", &repositories.SessionListCursor{UpdatedAt: time.Now().UTC(), ID: sessionBody.ID})
	if err != nil {
		t.Fatal(err)
	}
	crossUserToken := requestAsUser(t, http.MethodGet, server.URL+"/v1/sessions?pageToken="+token.(string), "", "cross-user-token", "", "user:2")
	crossUserToken.Body.Close()
	if crossUserToken.StatusCode != http.StatusBadRequest {
		t.Fatalf("cross-user token status=%d, want 400", crossUserToken.StatusCode)
	}
}

func TestPlannerHistoryUsesLatestSixPersistedQueryPlans(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "ai-core.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	notifier := inmemory.New()
	clock, generator := clockadapter.NewSystem(), idadapter.New()
	workflow := workflows.RunAnalysisWorkflow{Store: store, Notifier: notifier, Runtime: httpRuntime{}, IDs: generator, Clock: clock}
	planner := &httpPlanner{}
	api := &API{Store: store, Notifier: notifier, Commands: commands.New(store, notifier, workflow, generator, clock, planner)}
	server := httptest.NewServer(NewHandler(api))
	defer server.Close()

	sessionResponse := request(t, http.MethodPost, server.URL+"/v1/sessions", `{"title":"Bounded history"}`, "history-session", "")
	var sessionBody struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(sessionResponse.Body).Decode(&sessionBody); err != nil {
		t.Fatal(err)
	}
	sessionResponse.Body.Close()

	for index := 0; index < 8; index++ {
		message := "request-" + string(rune('0'+index))
		body, err := json.Marshal(map[string]any{"sessionId": sessionBody.ID, "message": message, "analysisContext": map[string]any{"datasourceUid": "prometheus-main"}})
		if err != nil {
			t.Fatal(err)
		}
		response := request(t, http.MethodPost, server.URL+"/v1/tasks", string(body), "history-task", "history-key-"+string(rune('0'+index)))
		if response.StatusCode != http.StatusAccepted {
			response.Body.Close()
			t.Fatalf("task %d response: %d", index, response.StatusCode)
		}
		var taskBody struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(response.Body).Decode(&taskBody); err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		waitTaskTerminal(t, store, taskBody.ID)
	}

	last := planner.requests[len(planner.requests)-1]
	if len(last.PreviousIntents) != 6 {
		t.Fatalf("previous intent count = %d, want 6", len(last.PreviousIntents))
	}
	for index, previous := range last.PreviousIntents {
		wantMessage := "request-" + string(rune('1'+index))
		if previous.Message != wantMessage || len(previous.Views) != 1 || previous.Views[0] != "cpu" || previous.RangeSeconds != 1800 || previous.StepSeconds != 10 {
			t.Fatalf("previous intent %d = %#v, want message %q and default CPU plan", index, previous, wantMessage)
		}
	}
}

type httpPlanner struct {
	calls    int
	failure  error
	requests []agent.IntentPlanRequest
}

func (p *httpPlanner) Plan(_ context.Context, _ requestcontext.Context, request agent.IntentPlanRequest) (agent.IntentPlan, error) {
	p.calls++
	p.requests = append(p.requests, request)
	if p.failure != nil {
		return agent.IntentPlan{}, p.failure
	}
	return agent.IntentPlan{Status: agent.IntentPlanned, Views: []string{"cpu"}}, nil
}

func hasHTTPTestCode(err error, code common.ErrorCode) bool {
	var domainErr *common.DomainError
	return errors.As(err, &domainErr) && domainErr.Code == code
}

func waitTaskTerminal(t *testing.T, store *storage.Store, taskID string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	var lastStatus any
	var lastErr error
	for time.Now().Before(deadline) {
		item, err := store.Tasks().Get(context.Background(), "org:1", taskID)
		lastErr = err
		lastStatus = item.Status
		if err == nil && (item.Status == "completed" || item.Status == "failed") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("task did not reach a terminal state: status=%v err=%v", lastStatus, lastErr)
}

func TestIncidentContractPlaceholdersFailClosed(t *testing.T) {
	server := httptest.NewServer(NewHandler(&API{}))
	defer server.Close()

	response := request(t, http.MethodGet, server.URL+"/v1/incidents", "", "request-incidents", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotImplemented)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != string(common.NotImplemented) {
		t.Fatalf("error code = %q", body.Error.Code)
	}
}

func request(t *testing.T, method, target, body, requestID, idempotencyKey string) *http.Response {
	return requestAsUser(t, method, target, body, requestID, idempotencyKey, "user:1")
}

func requestAsUser(t *testing.T, method, target, body, requestID, idempotencyKey, userID string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	addHeaders(req, requestID, idempotencyKey)
	req.Header.Set("X-MTB-User-ID", userID)
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func addHeaders(request *http.Request, requestID, idempotencyKey string) {
	request.Header.Set("X-MTB-Tenant-ID", "org:1")
	request.Header.Set("X-MTB-Org-ID", "1")
	request.Header.Set("X-MTB-User-ID", "user:1")
	request.Header.Set("X-MTB-Roles", "Viewer")
	request.Header.Set("X-MTB-Permissions", "datasources:query")
	request.Header.Set("X-Request-ID", requestID)
	request.Header.Set("X-Trace-ID", "trace-1")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
}

type httpRuntime struct{}

func (httpRuntime) Run(ctx context.Context, _ requestcontext.Context, request dto.AgentRunRequest, sink agent.EventSink) (dto.AgentRunResult, error) {
	if err := sink.Emit(ctx, dto.AgentEvent{Type: "assistant.message.delta", Payload: "fixed"}); err != nil {
		return dto.AgentRunResult{}, err
	}
	proposals := make([]dto.ChartProposal, 0, 3)
	for _, name := range []string{"CPU 使用率", "内存可用率", "系统负载"} {
		proposals = append(proposals, dto.ChartProposal{Title: name, Unit: "short", Query: chart.QuerySpec{RefID: "A", Expression: "node_load1", Legend: "{{instance}}", DatasourceUID: request.DatasourceUID, TimeRange: request.TimeRange, StepSeconds: request.QueryPlan.StepSeconds}, Execution: dto.QueryExecutionResult{Status: "success"}})
	}
	return dto.AgentRunResult{AssistantText: "fixed", Proposals: proposals}, nil
}
func (httpRuntime) Resume(context.Context, requestcontext.Context, dto.AgentResumeRequest, agent.EventSink) (dto.AgentRunResult, error) {
	return dto.AgentRunResult{}, nil
}
