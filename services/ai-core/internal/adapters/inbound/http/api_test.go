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

	body := `{"sessionId":"` + sessionBody.ID + `","message":"show node exporter","analysisContext":{"datasourceUid":"mock-prometheus","timeRange":{"relativeDuration":"30m"}}}`
	taskResponse := request(t, http.MethodPost, server.URL+"/v1/tasks", body, "request-task", "task-key")
	if taskResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("task response: %d", taskResponse.StatusCode)
	}
	var taskBody struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(taskResponse.Body).Decode(&taskBody); err != nil {
		t.Fatal(err)
	}
	taskResponse.Body.Close()
	if taskBody.ID == "" {
		t.Fatal("task id is missing")
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
		proposals = append(proposals, dto.ChartProposal{Title: name, Unit: "short", Query: chart.QuerySpec{RefID: "A", Expression: "node_load1", Legend: "{{instance}}", DatasourceUID: request.DatasourceUID, TimeRange: request.TimeRange}, Execution: dto.QueryExecutionResult{Status: "success"}})
	}
	return dto.AgentRunResult{AssistantText: "fixed", Proposals: proposals}, nil
}
func (httpRuntime) Resume(context.Context, requestcontext.Context, dto.AgentResumeRequest, agent.EventSink) (dto.AgentRunResult, error) {
	return dto.AgentRunResult{}, nil
}
