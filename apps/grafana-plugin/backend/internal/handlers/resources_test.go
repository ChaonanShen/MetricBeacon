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
		_, _ = w.Write([]byte(`{"id":"task_1","sessionId":"session_1","status":"created","inputMessageId":"message_1","datasourceUid":"prometheus-main","timeRange":{"from":"2026-07-13T12:00:00Z","to":"2026-07-13T12:30:00Z"},"latestSequence":1,"error":null,"createdAt":"2026-07-13T12:00:00Z","startedAt":null,"completedAt":null,"updatedAt":"2026-07-13T12:00:00Z","version":1}`))
	}))
	defer upstream.Close()
	client, err := generated.NewClient(upstream.URL, generated.WithHTTPClient(&http.Client{Timeout: time.Second}))
	if err != nil {
		t.Fatal(err)
	}
	handler := &ResourceHandler{Client: client, MaxResponse: 1 << 20}
	sender := &captureSender{}
	request := &backend.CallResourceRequest{Method: http.MethodPost, Path: "tasks", Headers: map[string][]string{"X-MTB-Tenant-ID": {"forged"}, "Idempotency-Key": {"key-1"}}, Body: []byte(`{"sessionId":"session_1","message":"show metrics","analysisContext":{"datasourceUid":"prometheus-main","timeRange":{"relativeDuration":"30m"}}}`), PluginContext: backend.PluginContext{OrgID: 42, User: &backend.User{Login: "grafana-user", Role: "Viewer"}}}
	if err := handler.CallResource(context.Background(), request, sender); err != nil {
		t.Fatal(err)
	}
	if len(sender.responses) != 1 || sender.responses[0].Status != http.StatusAccepted {
		t.Fatalf("responses: %#v", sender.responses)
	}
	if received.Get("X-MTB-Tenant-ID") != "org:42" || received.Get("X-MTB-User-ID") != "grafana-user" || received.Get("X-MTB-Permissions") != "datasources:query,incidents:read" {
		t.Fatalf("untrusted headers leaked or identity missing: %#v", received)
	}
}

func TestResourceHandlerProxiesOrgIncidentsAndAdminApprovalWithTrustedIdentity(t *testing.T) {
	requests := make(map[string]*http.Request)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.Method+" "+r.URL.Path] = r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/incidents":
			_, _ = w.Write([]byte(`{"items":[],"nextPageToken":null}`))
		case "GET /v1/tasks/task_1/approval":
			_, _ = w.Write([]byte(`{"id":"approval_1"}`))
		case "POST /v1/tasks/task_1/approval":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"approval_1","status":"approved"}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer upstream.Close()
	client, _ := generated.NewClient(upstream.URL)
	handler := &ResourceHandler{Client: client}
	digest := "sha256:" + strings.Repeat("a", 64)

	requestsToSend := []*backend.CallResourceRequest{
		{Method: http.MethodGet, Path: "incidents", URL: "?pageSize=20&pageToken=incident-token", Headers: map[string][]string{"X-MTB-Org-ID": {"forged"}}, PluginContext: backend.PluginContext{OrgID: 7, User: &backend.User{Login: "viewer", Role: "Viewer"}}},
		{Method: http.MethodGet, Path: "tasks/task_1/approval", Headers: map[string][]string{"X-MTB-Roles": {"Admin"}}, PluginContext: backend.PluginContext{OrgID: 7, User: &backend.User{Login: "viewer", Role: "Viewer"}}},
		{Method: http.MethodPost, Path: "tasks/task_1/approval", Headers: map[string][]string{"Idempotency-Key": {"approve-key"}, "X-MTB-Org-ID": {"forged"}, "X-MTB-Roles": {"Viewer"}}, Body: []byte(`{"decision":"approve","reason":"reviewed","expectedTaskVersion":5,"expectedApprovalVersion":1,"intentDigest":"` + digest + `"}`), PluginContext: backend.PluginContext{OrgID: 7, User: &backend.User{Login: "admin", Role: "Admin"}}},
	}
	for _, request := range requestsToSend {
		sender := &captureSender{}
		if err := handler.CallResource(context.Background(), request, sender); err != nil {
			t.Fatal(err)
		}
		if len(sender.responses) != 1 || sender.responses[0].Status < 200 || sender.responses[0].Status >= 300 {
			t.Fatalf("request=%s responses=%#v", request.Path, sender.responses)
		}
	}
	incidentRequest := requests["GET /v1/incidents"]
	if incidentRequest == nil || incidentRequest.URL.RawQuery != "pageSize=20&pageToken=incident-token" || incidentRequest.Header.Get("X-MTB-Tenant-ID") != "org:7" || incidentRequest.Header.Get("X-MTB-Permissions") != "datasources:query,incidents:read" {
		t.Fatalf("incident request=%#v", incidentRequest)
	}
	getRequest := requests["GET /v1/tasks/task_1/approval"]
	if getRequest == nil || getRequest.Header.Get("X-MTB-Roles") != "Viewer" || getRequest.Header.Get("X-MTB-Permissions") != "datasources:query,incidents:read" {
		t.Fatalf("approval GET=%#v", getRequest)
	}
	postRequest := requests["POST /v1/tasks/task_1/approval"]
	if postRequest == nil || postRequest.Header.Get("X-MTB-Roles") != "Admin" || postRequest.Header.Get("X-MTB-Permissions") != "datasources:query,incidents:read,incidents:approve" || postRequest.Header.Get("Idempotency-Key") != "approve-key" {
		t.Fatalf("approval POST=%#v", postRequest)
	}
}

func TestResourceHandlerRejectsNonAdminApprovalBeforeUpstream(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { upstreamCalls++ }))
	defer upstream.Close()
	client, _ := generated.NewClient(upstream.URL)
	handler := &ResourceHandler{Client: client}
	request := &backend.CallResourceRequest{Method: http.MethodPost, Path: "tasks/task_1/approval", Headers: map[string][]string{"Idempotency-Key": {"key"}, "X-MTB-Roles": {"Admin"}}, Body: []byte(`{}`), PluginContext: backend.PluginContext{OrgID: 1, User: &backend.User{Login: "viewer", Role: "Viewer"}}}
	sender := &captureSender{}
	if err := handler.CallResource(context.Background(), request, sender); err != nil {
		t.Fatal(err)
	}
	if upstreamCalls != 0 || len(sender.responses) != 1 || sender.responses[0].Status != http.StatusForbidden || !strings.Contains(string(sender.responses[0].Body), "Admin") {
		t.Fatalf("calls=%d responses=%#v", upstreamCalls, sender.responses)
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
		case "/v1/sessions":
			_, _ = w.Write([]byte(`{"items":[],"nextPageToken":"next-session"}`))
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
		{Method: http.MethodGet, Path: "sessions", URL: "?pageSize=20&pageToken=session-token", Headers: map[string][]string{"X-MTB-Tenant-ID": {"forged"}}, PluginContext: pluginContext},
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
		"/v1/sessions":                    "pageSize=20&pageToken=session-token",
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

func TestResourceHandlerRejectsInvalidSessionPageParameters(t *testing.T) {
	handler := &ResourceHandler{Client: mustClient(t, "http://127.0.0.1:1")}
	request := &backend.CallResourceRequest{Method: http.MethodGet, Path: "sessions", URL: "?pageSize=51", PluginContext: backend.PluginContext{OrgID: 1, User: &backend.User{Login: "grafana-user", Role: "Viewer"}}}
	sender := &captureSender{}
	if err := handler.CallResource(context.Background(), request, sender); err != nil {
		t.Fatal(err)
	}
	if len(sender.responses) != 1 || sender.responses[0].Status != http.StatusBadRequest || !strings.Contains(string(sender.responses[0].Body), "Invalid session page parameters") {
		t.Fatalf("responses: %#v", sender.responses)
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
