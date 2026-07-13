package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	generated "mini-torchbearing.local/packages/generated-clients/go"
)

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

type captureSender struct {
	responses []*backend.CallResourceResponse
}

func (s *captureSender) Send(response *backend.CallResourceResponse) error {
	s.responses = append(s.responses, response)
	return nil
}
