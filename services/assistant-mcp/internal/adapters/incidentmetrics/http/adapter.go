package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	stdhttp "net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/incidentmetrics"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

const maxBodyBytes = 64 << 10

var registeredQueries = map[string]string{
	"accepted":  "sum(increase(mtb_demo_orders_received_total{result=\"accepted\"}[30s]))",
	"completed": "sum(increase(mtb_demo_orders_completed_total{result=\"completed\"}[30s]))",
	"depth":     "max(mtb_demo_order_queue_depth)",
	"oldest":    "max(mtb_demo_order_queue_oldest_age_seconds)",
}

type Adapter struct {
	endpoint *url.URL
	client   *stdhttp.Client
	now      func() time.Time
}

func New(endpoint string, timeout time.Duration) (*Adapter, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || timeout <= 0 {
		return nil, fmt.Errorf("Incident metrics endpoint and timeout are invalid")
	}
	return newWithClient(parsed, &stdhttp.Client{Timeout: timeout, CheckRedirect: func(*stdhttp.Request, []*stdhttp.Request) error { return stdhttp.ErrUseLastResponse }}, time.Now)
}

func newWithClient(endpoint *url.URL, client *stdhttp.Client, now func() time.Time) (*Adapter, error) {
	if endpoint == nil || client == nil || now == nil {
		return nil, fmt.Errorf("Incident metrics adapter is not configured")
	}
	copy := *endpoint
	copy.Path = strings.TrimRight(copy.Path, "/")
	return &Adapter{endpoint: &copy, client: client, now: now}, nil
}

func (a *Adapter) GetRecovery(ctx context.Context, _ requestcontext.Context) (incidentmetrics.Recovery, error) {
	observedAt := a.now().UTC()
	values := make(map[string]float64, len(registeredQueries))
	for key, query := range registeredQueries {
		value, err := a.query(ctx, query, observedAt)
		if err != nil {
			return incidentmetrics.Recovery{}, err
		}
		values[key] = value
	}
	return incidentmetrics.Recovery{WindowSeconds: 30, AcceptedDelta: values["accepted"], CompletedDelta: values["completed"], QueueDepth: values["depth"], OldestAgeSeconds: values["oldest"], ObservedAt: observedAt}, nil
}

func (a *Adapter) query(ctx context.Context, expression string, observedAt time.Time) (float64, error) {
	endpoint := *a.endpoint
	endpoint.Path += "/api/v1/query"
	query := endpoint.Query()
	query.Set("query", expression)
	query.Set("time", strconv.FormatFloat(float64(observedAt.UnixNano())/float64(time.Second), 'f', -1, 64))
	endpoint.RawQuery = query.Encode()
	request, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, runtime.NewError(runtime.InternalError, "registered recovery query could not be created", false)
	}
	response, err := a.client.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return 0, runtime.NewError(runtime.ToolTimeout, "Incident metrics query timed out", true)
		}
		return 0, runtime.NewError(runtime.DependencyUnavailable, "Prometheus recovery view is unavailable", true)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return 0, runtime.NewError(runtime.DependencyUnavailable, "Prometheus recovery view returned an unsuccessful response", response.StatusCode >= 500)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil || len(contents) > maxBodyBytes {
		return 0, runtime.NewError(runtime.SchemaValidationFailed, "Prometheus recovery response exceeds its bound", false)
	}
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value []json.RawMessage `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if json.Unmarshal(contents, &payload) != nil || payload.Status != "success" || payload.Data.ResultType != "vector" || len(payload.Data.Result) != 1 || len(payload.Data.Result[0].Value) != 2 {
		return 0, runtime.NewError(runtime.SchemaValidationFailed, "Prometheus recovery response is invalid", false)
	}
	var text string
	if json.Unmarshal(payload.Data.Result[0].Value[1], &text) != nil {
		return 0, runtime.NewError(runtime.SchemaValidationFailed, "Prometheus recovery sample is invalid", false)
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, runtime.NewError(runtime.SchemaValidationFailed, "Prometheus recovery sample is outside bounds", false)
	}
	return value, nil
}
