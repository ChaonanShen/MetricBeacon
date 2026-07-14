package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	generated "mini-torchbearing.local/packages/generated-clients/go"
)

func TestResourceHandlerUsesProvisionedAppEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sessions" {
			t.Errorf("path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"session_1"}`))
	}))
	defer upstream.Close()

	var configuredEndpoint string
	handler := &ResourceHandler{
		NewClient: func(endpoint string) (generated.ClientInterface, error) {
			configuredEndpoint = endpoint
			return generated.NewClient(endpoint)
		},
	}
	settings, err := json.Marshal(appSettings{AICoreEndpoint: upstream.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	request := &backend.CallResourceRequest{
		Method: http.MethodPost,
		Path:   "sessions",
		Body:   []byte(`{"title":"Provisioned session"}`),
		PluginContext: backend.PluginContext{
			OrgID:               1,
			User:                &backend.User{Login: "grafana-user", Role: "Viewer"},
			AppInstanceSettings: &backend.AppInstanceSettings{JSONData: settings},
		},
	}
	sender := &captureSender{}
	if err := handler.CallResource(context.Background(), request, sender); err != nil {
		t.Fatal(err)
	}
	if configuredEndpoint != upstream.URL {
		t.Fatalf("configured endpoint = %q, want %q", configuredEndpoint, upstream.URL)
	}
	if len(sender.responses) != 1 || sender.responses[0].Status != http.StatusCreated {
		t.Fatalf("responses: %#v", sender.responses)
	}
}

func TestResourceHandlerRejectsMissingProvisionedEndpoint(t *testing.T) {
	handler := &ResourceHandler{NewClient: func(endpoint string) (generated.ClientInterface, error) {
		return generated.NewClient(endpoint)
	}}
	request := &backend.CallResourceRequest{
		Method: http.MethodPost,
		Path:   "sessions",
		Body:   []byte(`{"title":"Missing endpoint"}`),
		PluginContext: backend.PluginContext{
			OrgID:               1,
			User:                &backend.User{Login: "grafana-user", Role: "Viewer"},
			AppInstanceSettings: &backend.AppInstanceSettings{JSONData: json.RawMessage(`{}`)},
		},
	}
	sender := &captureSender{}
	if err := handler.CallResource(context.Background(), request, sender); err != nil {
		t.Fatal(err)
	}
	if len(sender.responses) != 1 || sender.responses[0].Status != http.StatusServiceUnavailable {
		t.Fatalf("responses: %#v", sender.responses)
	}
	if !strings.Contains(string(sender.responses[0].Body), "AI Core endpoint is not configured") {
		t.Fatalf("body: %s", sender.responses[0].Body)
	}
}

func TestResourceHandlerUsesGrafanaIdentityAndGeneratedClient(t *testing.T) {
	var received http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Clone()
		if r.URL.Path != "/v1/tasks" {
			t.Errorf("path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"task_1","sessionId":"session_1","status":"created","inputMessageId":"message_1","datasourceUid":"mock-prometheus","timeRange":{"from":"2026-07-13T12:00:00Z","to":"2026-07-13T12:30:00Z"},"latestSequence":1,"error":null,"createdAt":"2026-07-13T12:00:00Z","startedAt":null,"completedAt":null,"updatedAt":"2026-07-13T12:00:00Z","version":1}`))
	}))
	defer upstream.Close()
	client, err := generated.NewClient(upstream.URL, generated.WithHTTPClient(&http.Client{Timeout: time.Second}))
	if err != nil {
		t.Fatal(err)
	}
	handler := &ResourceHandler{Client: client, MaxResponse: 1 << 20}
	sender := &captureSender{}
	request := &backend.CallResourceRequest{Method: http.MethodPost, Path: "tasks", Headers: map[string][]string{"X-MTB-Tenant-ID": {"forged"}, "Idempotency-Key": {"key-1"}}, Body: []byte(`{"sessionId":"session_1","message":"show metrics","analysisContext":{"datasourceUid":"mock-prometheus","timeRange":{"relativeDuration":"30m"}}}`), PluginContext: backend.PluginContext{OrgID: 42, User: &backend.User{Login: "grafana-user", Role: "Viewer"}}}
	if err := handler.CallResource(context.Background(), request, sender); err != nil {
		t.Fatal(err)
	}
	if len(sender.responses) != 1 || sender.responses[0].Status != http.StatusAccepted {
		t.Fatalf("responses: %#v", sender.responses)
	}
	if received.Get("X-MTB-Tenant-ID") != "org:42" || received.Get("X-MTB-User-ID") != "grafana-user" || received.Get("X-MTB-Permissions") != "datasources:query" {
		t.Fatalf("untrusted headers leaked or identity missing: %#v", received)
	}
}

func TestResourceHandlerStreamsSSEBytesWithoutRewriting(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks/task_1/events" || r.URL.Query().Get("afterSequence") != "3" {
			t.Errorf("stream request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("id: 4\nevent: task.created\ndata: {\"sequence\":4}\n\n"))
	}))
	defer upstream.Close()
	client, err := generated.NewClient(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler := &ResourceHandler{Client: client}
	sender := &captureSender{}
	request := &backend.CallResourceRequest{Method: http.MethodGet, Path: "tasks/task_1/events", URL: "?afterSequence=3", PluginContext: backend.PluginContext{OrgID: 1, User: &backend.User{Login: "grafana-user", Role: "Viewer"}}}
	if err := handler.CallResource(context.Background(), request, sender); err != nil {
		t.Fatal(err)
	}
	var joined strings.Builder
	for _, response := range sender.responses {
		joined.Write(response.Body)
	}
	if joined.String() != "id: 4\nevent: task.created\ndata: {\"sequence\":4}\n\n" {
		t.Fatalf("SSE bytes changed: %q", joined.String())
	}
}

func TestResourceHandlerProxiesHistoryAndFiniteReplayWithGrafanaIdentity(t *testing.T) {
	requests := make(map[string]*http.Request)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path] = r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/sessions/session_1/messages":
			_, _ = w.Write([]byte(`{"items":[],"nextPageToken":"next-message"}`))
		case "/v1/sessions/session_1/tasks":
			_, _ = w.Write([]byte(`{"items":[],"nextPageToken":"next-task"}`))
		case "/v1/tasks/task_1/events/replay":
			_, _ = w.Write([]byte(`{"items":[],"targetSequence":3,"nextPageToken":null}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()
	client, err := generated.NewClient(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler := &ResourceHandler{Client: client, MaxResponse: 4 << 20}
	pluginContext := backend.PluginContext{OrgID: 7, User: &backend.User{Login: "grafana-user", Role: "Viewer"}}
	for _, request := range []*backend.CallResourceRequest{
		{Method: http.MethodGet, Path: "sessions/session_1/messages", URL: "?pageSize=50&pageToken=message-token", Headers: map[string][]string{"X-MTB-Tenant-ID": {"forged"}}, PluginContext: pluginContext},
		{Method: http.MethodGet, Path: "sessions/session_1/tasks", URL: "?pageSize=20&pageToken=task-token", Headers: map[string][]string{"X-MTB-User-ID": {"forged"}}, PluginContext: pluginContext},
		{Method: http.MethodGet, Path: "tasks/task_1/events/replay", URL: "?pageSize=200&pageToken=replay-token", Headers: map[string][]string{"X-MTB-Org-ID": {"forged"}}, PluginContext: pluginContext},
	} {
		sender := &captureSender{}
		if err := handler.CallResource(context.Background(), request, sender); err != nil {
			t.Fatal(err)
		}
		if len(sender.responses) != 1 || sender.responses[0].Status != http.StatusOK {
			t.Fatalf("responses for %s: %#v", request.Path, sender.responses)
		}
	}
	for path, wantQuery := range map[string]string{
		"/v1/sessions/session_1/messages": "pageSize=50&pageToken=message-token",
		"/v1/sessions/session_1/tasks":    "pageSize=20&pageToken=task-token",
		"/v1/tasks/task_1/events/replay":  "pageSize=200&pageToken=replay-token",
	} {
		request := requests[path]
		if request == nil || request.URL.RawQuery != wantQuery {
			t.Fatalf("request for %s = %#v, want query %q", path, request, wantQuery)
		}
		if request.Header.Get("X-MTB-Tenant-ID") != "org:7" || request.Header.Get("X-MTB-User-ID") != "grafana-user" {
			t.Fatalf("Grafana identity was not used for %s: %#v", path, request.Header)
		}
	}
}

func TestResourceHandlerRejectsInvalidFiniteReplayParameters(t *testing.T) {
	handler := &ResourceHandler{Client: mustClient(t, "http://127.0.0.1:1")}
	request := &backend.CallResourceRequest{Method: http.MethodGet, Path: "tasks/task_1/events/replay", URL: "?afterSequence=1&pageToken=cursor", PluginContext: backend.PluginContext{OrgID: 1, User: &backend.User{Login: "grafana-user", Role: "Viewer"}}}
	sender := &captureSender{}
	if err := handler.CallResource(context.Background(), request, sender); err != nil {
		t.Fatal(err)
	}
	if len(sender.responses) != 1 || sender.responses[0].Status != http.StatusBadRequest || !strings.Contains(string(sender.responses[0].Body), "Invalid event replay parameters") {
		t.Fatalf("responses: %#v", sender.responses)
	}
}

func TestResourceHandlerUsesDedicatedClientForSSE(t *testing.T) {
	var unaryCalls, streamCalls int
	handler := &ResourceHandler{
		NewClient: func(endpoint string) (generated.ClientInterface, error) {
			unaryCalls++
			return generated.NewClient("http://127.0.0.1:1")
		},
		NewStreamClient: func(endpoint string) (generated.ClientInterface, error) {
			streamCalls++
			return generated.NewClient("http://127.0.0.1:1")
		},
	}
	settings, err := json.Marshal(appSettings{AICoreEndpoint: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	request := &backend.CallResourceRequest{Method: http.MethodGet, Path: "tasks/task_1/events", PluginContext: backend.PluginContext{OrgID: 1, User: &backend.User{Login: "grafana-user", Role: "Viewer"}, AppInstanceSettings: &backend.AppInstanceSettings{JSONData: settings}}}
	sender := &captureSender{}
	if err := handler.CallResource(context.Background(), request, sender); err != nil {
		t.Fatal(err)
	}
	if streamCalls != 1 || unaryCalls != 0 {
		t.Fatalf("unary calls=%d, stream calls=%d", unaryCalls, streamCalls)
	}
}

func mustClient(t *testing.T, endpoint string) generated.ClientInterface {
	t.Helper()
	client, err := generated.NewClient(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

type captureSender struct {
	responses []*backend.CallResourceResponse
}

func (s *captureSender) Send(response *backend.CallResourceResponse) error {
	s.responses = append(s.responses, response)
	return nil
}
