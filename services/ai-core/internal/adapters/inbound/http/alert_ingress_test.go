package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/application/incidents"
)

func TestGrafanaAlertIngressVerifiesRawBodyAndMapsOnlyBoundedFacts(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	ingestor := &recordingAlertIngestor{seen: make(map[string]string)}
	handler := alertHandler(now, ingestor)
	body := validAlertBody(strings.Repeat("x", 201))

	response := signedAlertRequest(t, handler, now, body, alertRequestOptions{})
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Accepted, Duplicate int
		TaskIDs             []string `json:"taskIds"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Accepted != 1 || result.Duplicate != 0 || len(result.TaskIDs) != 1 {
		t.Fatalf("response: %#v", result)
	}
	inputs := ingestor.Inputs()
	if len(inputs) != 1 || inputs[0].SourceID != "demo-grafana" || inputs[0].AlertName != "OrderQueueBacklog" || inputs[0].ServiceRef != "order-demo" || inputs[0].RequestID != "request-alert" || inputs[0].TraceID != "trace-alert" {
		t.Fatalf("mapped input: %#v", inputs)
	}
	if _, leaked := inputs[0].Labels["long_transport_label"]; leaked {
		t.Fatalf("transport-only long label leaked into durable facts: %#v", inputs[0].Labels)
	}
	if _, leaked := inputs[0].Labels["groundTruth"]; leaked {
		t.Fatal("ground truth leaked into Incident input")
	}

	retry := signedAlertRequest(t, handler, now, body, alertRequestOptions{})
	if retry.Code != http.StatusAccepted {
		t.Fatalf("retry status=%d body=%s", retry.Code, retry.Body.String())
	}
	var retried struct {
		Accepted, Duplicate int
		TaskIDs             []string `json:"taskIds"`
	}
	_ = json.Unmarshal(retry.Body.Bytes(), &retried)
	if retried.Accepted != 0 || retried.Duplicate != 1 || len(retried.TaskIDs) != 1 || retried.TaskIDs[0] != result.TaskIDs[0] {
		t.Fatalf("retry response: %#v", retried)
	}
}

func TestGrafanaAlertIngressRejectsAuthenticationFailuresBeforeParsing(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	body := validAlertBody("")
	for name, options := range map[string]alertRequestOptions{
		"wrong source":      {source: "other"},
		"stale timestamp":   {timestamp: now.Add(-6 * time.Minute)},
		"future timestamp":  {timestamp: now.Add(6 * time.Minute)},
		"wrong signature":   {signature: strings.Repeat("0", 64)},
		"uppercase digest":  {uppercaseSignature: true},
		"tampered raw body": {signedBody: validAlertBody("signed"), body: body},
	} {
		t.Run(name, func(t *testing.T) {
			ingestor := &recordingAlertIngestor{seen: make(map[string]string)}
			response := signedAlertRequest(t, alertHandler(now, ingestor), now, body, options)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if len(ingestor.Inputs()) != 0 {
				t.Fatal("unauthenticated request reached application service")
			}
		})
	}
}

func TestGrafanaAlertIngressRejectsUnknownMissingAndOversizedPayloads(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"ground truth":        strings.Replace(validAlertBody(""), `"fingerprint":"fingerprint-1"`, `"fingerprint":"fingerprint-1","groundTruth":"worker_stopped"`, 1),
		"missing annotations": strings.Replace(validAlertBody(""), `"annotations":{},`, "", 1),
		"status mismatch":     strings.Replace(validAlertBody(""), `"status":"firing","labels"`, `"status":"resolved","labels"`, 1),
		"wrong org":           strings.Replace(validAlertBody(""), `"orgId":1`, `"orgId":2`, 1),
		"too many alerts":     `{"receiver":"ai-core","status":"firing","orgId":1,"alerts":[]}`,
		"oversized metadata":  strings.Replace(validAlertBody(""), `"orgId":1`, `"orgId":1,"message":"`+strings.Repeat("m", 2001)+`"`, 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			ingestor := &recordingAlertIngestor{seen: make(map[string]string)}
			response := signedAlertRequest(t, alertHandler(now, ingestor), now, body, alertRequestOptions{})
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if len(ingestor.Inputs()) != 0 {
				t.Fatal("invalid payload reached application service")
			}
		})
	}

	ingestor := &recordingAlertIngestor{seen: make(map[string]string)}
	oversized := strings.Repeat("x", maxAlertWebhookBytes+1)
	response := signedAlertRequest(t, alertHandler(now, ingestor), now, oversized, alertRequestOptions{})
	if response.Code != http.StatusBadRequest || len(ingestor.Inputs()) != 0 {
		t.Fatalf("oversized status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGrafanaAlertIngressAcceptsClockWindowBoundaries(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	for _, timestamp := range []time.Time{now.Add(-5 * time.Minute), now.Add(5 * time.Minute)} {
		ingestor := &recordingAlertIngestor{seen: make(map[string]string)}
		response := signedAlertRequest(t, alertHandler(now, ingestor), now, validAlertBody(""), alertRequestOptions{timestamp: timestamp})
		if response.Code != http.StatusAccepted || len(ingestor.Inputs()) != 1 {
			t.Fatalf("timestamp=%s status=%d body=%s", timestamp, response.Code, response.Body.String())
		}
	}
}

type alertRequestOptions struct {
	source             string
	timestamp          time.Time
	signature          string
	uppercaseSignature bool
	signedBody         string
	body               string
}

func signedAlertRequest(t *testing.T, handler http.Handler, now time.Time, defaultBody string, options alertRequestOptions) *httptest.ResponseRecorder {
	t.Helper()
	body := defaultBody
	if options.body != "" {
		body = options.body
	}
	timestamp := options.timestamp
	if timestamp.IsZero() {
		timestamp = now
	}
	source := options.source
	if source == "" {
		source = "demo-grafana"
	}
	signedBody := body
	if options.signedBody != "" {
		signedBody = options.signedBody
	}
	signature := options.signature
	if signature == "" {
		mac := hmac.New(sha256.New, []byte("0123456789abcdef-test-secret"))
		_, _ = mac.Write([]byte(strconv.FormatInt(timestamp.Unix(), 10) + ":" + signedBody))
		signature = hex.EncodeToString(mac.Sum(nil))
	}
	if options.uppercaseSignature {
		signature = strings.ToUpper(signature)
	}
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/alerts/grafana", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-MTB-Alert-Source", source)
	request.Header.Set("X-MTB-Alert-Timestamp", strconv.FormatInt(timestamp.Unix(), 10))
	request.Header.Set("X-Grafana-Alerting-Signature", signature)
	request.Header.Set("X-Request-ID", "request-alert")
	request.Header.Set("X-Trace-ID", "trace-alert")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func alertHandler(now time.Time, ingestor AlertIngestor) http.Handler {
	return NewHandler(&API{Incidents: ingestor, AlertIngress: AlertIngressConfig{SourceID: "demo-grafana", OrgID: 1, HMACSecret: "0123456789abcdef-test-secret", MaxClockSkew: 5 * time.Minute, CurrentTime: func() time.Time { return now }}})
}

func validAlertBody(longLabel string) string {
	extra := ""
	if longLabel != "" {
		extra = `,"long_transport_label":"` + longLabel + `"`
	}
	return `{"receiver":"ai-core","status":"firing","orgId":1,"alerts":[{"status":"firing","labels":{"alertname":"OrderQueueBacklog","service_ref":"order-demo","severity":"warning"` + extra + `},"annotations":{},"startsAt":"2026-07-16T11:59:00Z","endsAt":"0001-01-01T00:00:00Z","fingerprint":"fingerprint-1","values":{"queue_depth":9},"generatorURL":"http://grafana/internal"}]}`
}

type recordingAlertIngestor struct {
	mu     sync.Mutex
	seen   map[string]string
	inputs []incidents.Alert
}

func (r *recordingAlertIngestor) Ingest(_ context.Context, input incidents.Alert) (incidents.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inputs = append(r.inputs, input)
	key := string(input.Status) + ":" + input.Fingerprint
	if taskID := r.seen[key]; taskID != "" {
		return incidents.Result{TaskID: taskID, Duplicate: true}, nil
	}
	taskID := "task-" + input.Fingerprint
	r.seen[key] = taskID
	return incidents.Result{TaskID: taskID, Accepted: true}, nil
}

func (r *recordingAlertIngestor) Inputs() []incidents.Alert {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]incidents.Alert(nil), r.inputs...)
}
