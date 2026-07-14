package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
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
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
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
	api := &API{Store: store, Notifier: notifier, Commands: commands.New(store, notifier, workflow, generator, clock)}
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
	if taskBody.ID == "" || len(taskBody.QueryPlan.Views) != 0 || taskBody.QueryPlan.StepSeconds != 10 || taskBody.QueryPlan.CPURateWindowSeconds != 60 {
		t.Fatalf("task id or resolved query plan is missing: %#v", taskBody)
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

	conflictBody := strings.Replace(body, "show node exporter", "different request", 1)
	conflictResponse := request(t, http.MethodPost, server.URL+"/v1/tasks", conflictBody, "request-task-conflict", "task-key")
	defer conflictResponse.Body.Close()
	if conflictResponse.StatusCode != http.StatusConflict {
		t.Fatalf("different-body retry response: %d", conflictResponse.StatusCode)
	}

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

func request(t *testing.T, method, target, body, requestID, idempotencyKey string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	addHeaders(req, requestID, idempotencyKey)
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
