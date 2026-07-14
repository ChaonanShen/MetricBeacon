// Package http adapts the restricted node_exporter registry to Prometheus's
// read-only HTTP API. It deliberately does not expose an arbitrary PromQL or
// endpoint capability through the MCP boundary.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/adapters/prometheus/registry"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/prometheus"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

const (
	maxRange       = 6 * time.Hour
	maxBodyBytes   = 2 << 20
	maxSeries      = 20
	maxSamples     = 5000
	maxLabelValues = 20
	labelsLookback = 15 * time.Minute
	readinessWait  = 2 * time.Second
)

type Adapter struct {
	endpoint *url.URL
	client   *http.Client
	timeout  time.Duration
}

var _ prometheus.Port = (*Adapter)(nil)

func New(endpoint string, timeout time.Duration) (*Adapter, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("Prometheus endpoint must be an absolute HTTP URL")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("Prometheus timeout must be positive")
	}
	return NewWithHTTPClient(parsed, timeout, &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }})
}

// NewWithHTTPClient is an adapter-local test seam. Production construction
// uses New, which installs the redirect policy itself.
func NewWithHTTPClient(endpoint *url.URL, timeout time.Duration, client *http.Client) (*Adapter, error) {
	if endpoint == nil || endpoint.Scheme == "" || endpoint.Host == "" || client == nil || timeout <= 0 {
		return nil, fmt.Errorf("Prometheus HTTP adapter configuration is invalid")
	}
	copy := *endpoint
	copy.Path = strings.TrimRight(copy.Path, "/")
	return &Adapter{endpoint: &copy, timeout: timeout, client: client}, nil
}

func (a *Adapter) SearchMetrics(ctx context.Context, _ requestcontext.Context, request prometheus.SearchMetricsRequest) (prometheus.SearchMetricsResult, error) {
	if request.DatasourceUID != registry.DatasourceUID || strings.TrimSpace(request.Query) == "" || request.Limit < 1 || request.Limit > 100 {
		return prometheus.SearchMetricsResult{}, invalid("search metrics request is invalid")
	}
	candidates := make([]prometheus.MetricCandidate, 0, 4)
	for index, metric := range registeredMetricNames() {
		metadata, err := a.metadata(ctx, metric)
		if err != nil {
			return prometheus.SearchMetricsResult{}, err
		}
		if len(metadata) == 0 || !matchesSearch(request.Query, metric, metadata[0].Help) {
			continue
		}
		entry := metadata[0]
		description := strings.TrimSpace(entry.Help)
		if description == "" {
			description = "Prometheus node_exporter metric " + metric
		}
		candidates = append(candidates, prometheus.MetricCandidate{
			MetricName: metric, Type: metricType(entry.Type), Description: description,
			Labels: labelsForMetric(metric), Score: 1 - float64(index)/100,
			Sources: []prometheus.MetricSource{{Type: "prometheus_metadata", Reference: metric}},
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].MetricName < candidates[j].MetricName })
	if len(candidates) > request.Limit {
		candidates = candidates[:request.Limit]
	}
	return prometheus.SearchMetricsResult{Candidates: candidates}, nil
}

func (a *Adapter) GetMetricLabels(ctx context.Context, _ requestcontext.Context, request prometheus.GetMetricLabelsRequest) (prometheus.MetricLabelsResult, error) {
	if request.DatasourceUID != registry.DatasourceUID || !registry.IsRegisteredMetric(request.MetricName) {
		return prometheus.MetricLabelsResult{}, invalid("metric labels request is invalid")
	}
	values := url.Values{"match[]": {request.MetricName}}
	now := time.Now().UTC()
	values.Set("start", strconv.FormatFloat(float64(now.Add(-labelsLookback).UnixNano())/float64(time.Second), 'f', -1, 64))
	values.Set("end", strconv.FormatFloat(float64(now.UnixNano())/float64(time.Second), 'f', -1, 64))
	var response apiResponse[[]map[string]string]
	if err := a.postForm(ctx, "/api/v1/series", values, &response); err != nil {
		return prometheus.MetricLabelsResult{}, err
	}
	if !response.Success() {
		return prometheus.MetricLabelsResult{}, unavailable("Prometheus series response is invalid")
	}
	if len(response.Data) == 0 {
		return prometheus.MetricLabelsResult{}, runtime.NewError(runtime.ResourceNotFound, "registered metric is not available", false)
	}
	if len(response.Data) > maxSeries {
		return prometheus.MetricLabelsResult{}, invalid("Prometheus series response exceeds the configured limit")
	}
	samples := map[string]map[string]struct{}{}
	for _, series := range response.Data {
		for name, value := range series {
			if name == "__name__" {
				continue
			}
			if samples[name] == nil {
				samples[name] = map[string]struct{}{}
			}
			samples[name][value] = struct{}{}
		}
	}
	result := prometheus.MetricLabelsResult{MetricName: request.MetricName, SampleValues: make(map[string][]string, len(samples))}
	for name, distinct := range samples {
		values := make([]string, 0, len(distinct))
		for value := range distinct {
			values = append(values, value)
		}
		sort.Strings(values)
		if len(values) > maxLabelValues {
			values = values[:maxLabelValues]
		}
		result.LabelNames = append(result.LabelNames, name)
		result.SampleValues[name] = values
	}
	sort.Strings(result.LabelNames)
	if len(result.LabelNames) == 0 {
		return prometheus.MetricLabelsResult{}, runtime.NewError(runtime.ResourceNotFound, "registered metric has no labels", false)
	}
	return result, nil
}

func (a *Adapter) Query(ctx context.Context, _ requestcontext.Context, request prometheus.QueryRequest) (prometheus.QueryResult, error) {
	if request.DatasourceUID != registry.DatasourceUID || !request.Start.Before(request.End) || request.End.Sub(request.Start) > maxRange || request.StepSeconds < 1 || request.StepSeconds > 3600 || (request.Mode != prometheus.ModeValidate && request.Mode != prometheus.ModeExecute) {
		return prometheus.QueryResult{}, invalid("Prometheus query request is invalid")
	}
	definition, err := registry.Validate(request.Expression)
	if err != nil {
		return prometheus.QueryResult{}, invalid("PromQL expression is outside the node_exporter registry")
	}
	validation := prometheus.Validation{Valid: true, Errors: []string{}, Warnings: []string{}, MetricNames: append([]string{}, definition.MetricNames...), LabelNames: append([]string{}, definition.LabelNames...), CanonicalExpression: definition.CanonicalExpression}
	if request.Mode == prometheus.ModeValidate {
		return prometheus.QueryResult{Validation: validation, Status: "success", ResultType: "matrix", Series: []prometheus.Series{}, DurationMS: 0, Warnings: []string{}}, nil
	}
	started := time.Now()
	values := url.Values{"query": {definition.CanonicalExpression}}
	values.Set("start", strconv.FormatFloat(float64(request.Start.UTC().UnixNano())/float64(time.Second), 'f', -1, 64))
	values.Set("end", strconv.FormatFloat(float64(request.End.UTC().UnixNano())/float64(time.Second), 'f', -1, 64))
	values.Set("step", strconv.Itoa(request.StepSeconds))
	var response apiResponse[queryData]
	if err := a.postForm(ctx, "/api/v1/query_range", values, &response); err != nil {
		return prometheus.QueryResult{}, err
	}
	if !response.Success() || response.Data.ResultType != "matrix" {
		return prometheus.QueryResult{}, unavailable("Prometheus query response is invalid")
	}
	if len(response.Data.Result) > maxSeries {
		return prometheus.QueryResult{}, invalid("Prometheus query response exceeds the configured series limit")
	}
	series, warnings, err := parseSeries(response.Data.Result)
	if err != nil {
		return prometheus.QueryResult{}, err
	}
	return prometheus.QueryResult{Validation: validation, Status: "success", ResultType: "matrix", Series: series, DurationMS: int(time.Since(started).Milliseconds()), Warnings: warnings}, nil
}

// Ready performs the Prometheus-specific readiness probe. /healthz remains a
// process probe and must not call this method.
func (a *Adapter) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, readinessTimeout(a.timeout))
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.url("/-/ready").String(), nil)
	if err != nil {
		return unavailable("Prometheus readiness probe could not be created")
	}
	response, err := a.client.Do(request)
	if err != nil {
		return classifyTransportError(ctx, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return classifyStatus(response.StatusCode)
	}
	return nil
}

type metadata struct {
	Type string `json:"type"`
	Help string `json:"help"`
}
type apiResponse[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
}

func (r apiResponse[T]) Success() bool { return r.Status == "success" }

type queryData struct {
	ResultType string        `json:"resultType"`
	Result     []querySeries `json:"result"`
}
type querySeries struct {
	Metric map[string]string   `json:"metric"`
	Values [][]json.RawMessage `json:"values"`
}

func (a *Adapter) metadata(ctx context.Context, metric string) ([]metadata, error) {
	values := url.Values{"metric": {metric}}
	var response apiResponse[map[string][]metadata]
	if err := a.get(ctx, "/api/v1/metadata", values, &response); err != nil {
		return nil, err
	}
	if !response.Success() {
		return nil, unavailable("Prometheus metadata response is invalid")
	}
	return response.Data[metric], nil
}

func (a *Adapter) get(ctx context.Context, path string, values url.Values, destination any) error {
	endpoint := a.url(path)
	endpoint.RawQuery = values.Encode()
	requestContext, cancel := a.requestContext(ctx)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return unavailable("Prometheus request could not be created")
	}
	return a.do(request, destination)
}

func (a *Adapter) postForm(ctx context.Context, path string, values url.Values, destination any) error {
	requestContext, cancel := a.requestContext(ctx)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, a.url(path).String(), strings.NewReader(values.Encode()))
	if err != nil {
		return unavailable("Prometheus request could not be created")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return a.do(request, destination)
}

func (a *Adapter) do(request *http.Request, destination any) error {
	response, err := a.client.Do(request)
	if err != nil {
		return classifyTransportError(request.Context(), err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return classifyStatus(response.StatusCode)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return unavailable("Prometheus response could not be read")
	}
	if len(contents) > maxBodyBytes {
		return invalid("Prometheus response exceeds the configured size limit")
	}
	if err := json.Unmarshal(contents, destination); err != nil {
		return unavailable("Prometheus response is invalid")
	}
	return nil
}

func (a *Adapter) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, a.timeout)
}

func (a *Adapter) url(path string) *url.URL {
	copy := *a.endpoint
	copy.Path = strings.TrimRight(a.endpoint.Path, "/") + path
	return &copy
}

func parseSeries(source []querySeries) ([]prometheus.Series, []string, error) {
	result := make([]prometheus.Series, 0, len(source))
	warnings := []string{}
	samples := 0
	for _, item := range source {
		labels := make(map[string]string, len(item.Metric))
		name := item.Metric["instance"]
		for key, value := range item.Metric {
			if key != "__name__" {
				labels[key] = value
			}
		}
		if name == "" {
			name = item.Metric["__name__"]
		}
		if name == "" {
			return nil, nil, unavailable("Prometheus query response has an unnamed series")
		}
		series := prometheus.Series{Name: name, Labels: labels, Points: []prometheus.Point{}}
		for _, value := range item.Values {
			if len(value) != 2 {
				return nil, nil, unavailable("Prometheus query response has an invalid sample")
			}
			var timestamp float64
			var raw string
			if json.Unmarshal(value[0], &timestamp) != nil || json.Unmarshal(value[1], &raw) != nil || math.IsNaN(timestamp) || math.IsInf(timestamp, 0) {
				return nil, nil, unavailable("Prometheus query response has an invalid sample")
			}
			parsed, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return nil, nil, unavailable("Prometheus query response has an invalid sample")
			}
			samples++
			if samples > maxSamples {
				return nil, nil, invalid("Prometheus query response exceeds the configured sample limit")
			}
			if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
				warnings = appendWarning(warnings, "non-finite Prometheus samples were removed")
				continue
			}
			series.Points = append(series.Points, prometheus.Point{Timestamp: time.Unix(0, int64(timestamp*float64(time.Second))).UTC(), Value: parsed})
		}
		if len(series.Points) > 0 {
			result = append(result, series)
		}
	}
	return result, warnings, nil
}

func appendWarning(warnings []string, value string) []string {
	for _, item := range warnings {
		if item == value {
			return warnings
		}
	}
	return append(warnings, value)
}
func registeredMetricNames() []string {
	return []string{"node_cpu_seconds_total", "node_memory_MemAvailable_bytes", "node_memory_MemTotal_bytes", "node_load1"}
}
func labelsForMetric(metric string) []string {
	if metric == "node_cpu_seconds_total" {
		return []string{"instance", "mode"}
	}
	return []string{"instance"}
}
func metricType(value string) string {
	switch value {
	case "counter", "gauge", "histogram", "summary":
		return value
	default:
		return "unknown"
	}
}
func matchesSearch(query, metric, help string) bool {
	normalized := strings.ToLower(metric + " " + help)
	for _, token := range strings.Fields(strings.ToLower(query)) {
		if token != "node" && token != "exporter" && !strings.Contains(normalized, token) {
			return false
		}
	}
	return true
}
func readinessTimeout(timeout time.Duration) time.Duration {
	if timeout < readinessWait {
		return timeout
	}
	return readinessWait
}
func invalid(message string) error {
	return runtime.NewError(runtime.SchemaValidationFailed, message, false)
}
func unavailable(message string) error {
	return runtime.NewError(runtime.DependencyUnavailable, message, true)
}
func classifyStatus(status int) error {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return runtime.NewError(runtime.AdapterNotConfigured, "Prometheus credentials are not configured", false)
	}
	if status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout {
		return runtime.NewError(runtime.ToolTimeout, "Prometheus request timed out", true)
	}
	return unavailable("Prometheus is unavailable")
}
func classifyTransportError(ctx context.Context, err error) error {
	var networkError net.Error
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
		return runtime.NewError(runtime.ToolTimeout, "Prometheus request timed out", true)
	}
	return unavailable("Prometheus is unavailable")
}
