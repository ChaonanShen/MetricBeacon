package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/url"
	"strings"
	"time"

	generated "mini-torchbearing.local/packages/generated-clients/go/orderdemo"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/orderdemo"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

const maxResponseBytes = 64 << 10

var errResponseTooLarge = errors.New("order demo response exceeds limit")

type Adapter struct {
	readClient        *generated.ClientWithResponses
	remediationClient *generated.ClientWithResponses
}

var _ orderdemo.Port = (*Adapter)(nil)

func New(endpoint, readToken string, timeout time.Duration) (*Adapter, error) {
	return newAdapter(endpoint, readToken, "", timeout)
}

func NewWithRemediation(endpoint, readToken, remediationToken string, timeout time.Duration) (*Adapter, error) {
	if strings.TrimSpace(remediationToken) == "" {
		return nil, fmt.Errorf("order demo remediation token is required")
	}
	return newAdapter(endpoint, readToken, remediationToken, timeout)
}

func newAdapter(endpoint, readToken, remediationToken string, timeout time.Duration) (*Adapter, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("order demo endpoint must be an absolute HTTP URL")
	}
	if strings.TrimSpace(readToken) == "" || timeout <= 0 {
		return nil, fmt.Errorf("order demo read token and positive timeout are required")
	}
	httpClient := &boundedClient{client: &stdhttp.Client{Timeout: timeout, CheckRedirect: func(*stdhttp.Request, []*stdhttp.Request) error { return stdhttp.ErrUseLastResponse }}}
	readClient, err := generated.NewClientWithResponses(parsed.String(), generated.WithHTTPClient(httpClient), generated.WithRequestEditorFn(func(_ context.Context, request *stdhttp.Request) error {
		request.Header.Set("Authorization", "Bearer "+readToken)
		return nil
	}))
	if err != nil {
		return nil, err
	}
	result := &Adapter{readClient: readClient}
	if remediationToken != "" {
		result.remediationClient, err = generated.NewClientWithResponses(parsed.String(), generated.WithHTTPClient(httpClient), generated.WithRequestEditorFn(func(_ context.Context, request *stdhttp.Request) error {
			request.Header.Set("Authorization", "Bearer "+remediationToken)
			return nil
		}))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a *Adapter) GetRuntime(ctx context.Context, _ requestcontext.Context) (orderdemo.Runtime, error) {
	response, err := a.readClient.GetRuntimeWithResponse(ctx)
	if err != nil {
		return orderdemo.Runtime{}, classifyTransport(ctx, err)
	}
	if response.JSON200 == nil {
		return orderdemo.Runtime{}, classifyResponse(response.StatusCode())
	}
	value := response.JSON200
	serviceRef, ok := stringValue(value.ServiceRef)
	if !ok || serviceRef != "order-demo" || value.InstanceEpoch == "" || !value.SupervisorStatus.Valid() {
		return orderdemo.Runtime{}, invalidResponse()
	}
	return orderdemo.Runtime{ServiceRef: serviceRef, InstanceEpoch: value.InstanceEpoch, StartedAt: value.StartedAt, SupervisorStatus: string(value.SupervisorStatus)}, nil
}

func (a *Adapter) GetQueue(ctx context.Context, _ requestcontext.Context) (orderdemo.Queue, error) {
	response, err := a.readClient.GetQueueWithResponse(ctx)
	if err != nil {
		return orderdemo.Queue{}, classifyTransport(ctx, err)
	}
	if response.JSON200 == nil {
		return orderdemo.Queue{}, classifyResponse(response.StatusCode())
	}
	value := response.JSON200
	capacity, ok := intValue(value.Capacity)
	if !ok || capacity != 100 || value.Depth < 0 || value.Depth > capacity || value.OldestAgeSeconds < 0 {
		return orderdemo.Queue{}, invalidResponse()
	}
	return orderdemo.Queue{Depth: value.Depth, Capacity: capacity, OldestAgeSeconds: float64(value.OldestAgeSeconds), ObservedAt: value.ObservedAt}, nil
}

func (a *Adapter) GetWorker(ctx context.Context, _ requestcontext.Context) (orderdemo.Worker, error) {
	response, err := a.readClient.GetWorkerConfigWithResponse(ctx)
	if err != nil {
		return orderdemo.Worker{}, classifyTransport(ctx, err)
	}
	if response.JSON200 == nil {
		return orderdemo.Worker{}, classifyResponse(response.StatusCode())
	}
	value := response.JSON200
	serviceRef, ok := stringValue(value.ServiceRef)
	if !ok || serviceRef != "order-demo" || value.InstanceEpoch == "" || value.Version < 1 || !validConcurrency(value.ConfiguredConcurrency) || !validConcurrency(value.EffectiveConcurrency) || !validConcurrency(value.ActiveWorkers) || !validConcurrency(value.InflightOrders) {
		return orderdemo.Worker{}, invalidResponse()
	}
	return orderdemo.Worker{ServiceRef: serviceRef, InstanceEpoch: value.InstanceEpoch, ConfiguredConcurrency: value.ConfiguredConcurrency, EffectiveConcurrency: value.EffectiveConcurrency, ActiveWorkers: value.ActiveWorkers, InflightOrders: value.InflightOrders, Version: value.Version, ObservedAt: value.ObservedAt}, nil
}

func (a *Adapter) GetPolicy(ctx context.Context, _ requestcontext.Context) (orderdemo.Policy, error) {
	response, err := a.readClient.GetWorkerPolicyWithResponse(ctx)
	if err != nil {
		return orderdemo.Policy{}, classifyTransport(ctx, err)
	}
	if response.JSON200 == nil {
		return orderdemo.Policy{}, classifyResponse(response.StatusCode())
	}
	value := response.JSON200
	serviceRef, okService := stringValue(value.ServiceRef)
	expected, okExpected := intValue(value.ExpectedConcurrency)
	minimum, okMin := intValue(value.MinConcurrency)
	maximum, okMax := intValue(value.MaxConcurrency)
	version, okVersion := stringValue(value.Version)
	if !okService || serviceRef != "order-demo" || !okExpected || expected != 2 || !okMin || minimum != 1 || !okMax || maximum != 4 || !okVersion || version != "v1" || len(value.Digest) != 64 {
		return orderdemo.Policy{}, invalidResponse()
	}
	return orderdemo.Policy{ServiceRef: serviceRef, ExpectedConcurrency: expected, MinConcurrency: minimum, MaxConcurrency: maximum, Version: version, Digest: value.Digest}, nil
}

func (a *Adapter) GetRecentOutcomes(ctx context.Context, _ requestcontext.Context, request orderdemo.RecentRequest) ([]orderdemo.OrderOutcome, error) {
	if request.Limit < 1 || request.Limit > 20 || (request.Status != "" && !validStatus(request.Status)) {
		return nil, runtime.NewError(runtime.SchemaValidationFailed, "recent outcomes request is invalid", false)
	}
	params := &generated.ListRecentOrdersParams{Limit: &request.Limit}
	if request.Status != "" {
		status := generated.OrderStatus(request.Status)
		params.Status = &status
	}
	response, err := a.readClient.ListRecentOrdersWithResponse(ctx, params)
	if err != nil {
		return nil, classifyTransport(ctx, err)
	}
	if response.JSON200 == nil {
		return nil, classifyResponse(response.StatusCode())
	}
	if len(response.JSON200.Orders) > request.Limit || len(response.JSON200.Orders) > 20 {
		return nil, invalidResponse()
	}
	result := make([]orderdemo.OrderOutcome, 0, len(response.JSON200.Orders))
	for _, value := range response.JSON200.Orders {
		if value.Id == "" || !validStatus(string(value.Status)) {
			return nil, invalidResponse()
		}
		var reason *string
		if value.FailureReason != nil {
			converted := string(*value.FailureReason)
			if converted != "dependency_unavailable" && converted != "retry_exhausted" {
				return nil, invalidResponse()
			}
			reason = &converted
		}
		result = append(result, orderdemo.OrderOutcome{ID: value.Id, Status: string(value.Status), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, FailureReason: reason})
	}
	return result, nil
}

func (a *Adapter) GetOperation(ctx context.Context, _ requestcontext.Context, operationID string) (orderdemo.Operation, error) {
	if strings.TrimSpace(operationID) == "" || len(operationID) > 100 {
		return orderdemo.Operation{}, runtime.NewError(runtime.SchemaValidationFailed, "operation ID is invalid", false)
	}
	response, err := a.readClient.GetOperationWithResponse(ctx, operationID)
	if err != nil {
		return orderdemo.Operation{}, classifyTransport(ctx, err)
	}
	if response.JSON200 == nil {
		return orderdemo.Operation{}, classifyResponse(response.StatusCode())
	}
	return mapOperation(*response.JSON200)
}

func (a *Adapter) RestoreWorkerConcurrency(ctx context.Context, _ requestcontext.Context, request orderdemo.RemediationRequest) (orderdemo.Operation, error) {
	if a.remediationClient == nil {
		return orderdemo.Operation{}, runtime.NewError(runtime.AdapterNotConfigured, "order demo remediation adapter is not configured", false)
	}
	if request.OperationID == "" || request.InstanceEpoch == "" || request.ExpectedVersion < 1 || request.ExpectedConcurrency != 0 || request.NewConcurrency != 2 || !validDigest(request.IntentDigest) || request.ApprovalID == "" {
		return orderdemo.Operation{}, runtime.NewError(runtime.SchemaValidationFailed, "worker remediation request is invalid", false)
	}
	body := generated.UpdateWorkerConfigRequest{OperationId: request.OperationID, InstanceEpoch: request.InstanceEpoch, ExpectedVersion: request.ExpectedVersion, ExpectedConcurrency: request.ExpectedConcurrency, NewConcurrency: request.NewConcurrency, IntentDigest: request.IntentDigest, ApprovalId: request.ApprovalID}
	response, err := a.remediationClient.UpdateWorkerConfigWithResponse(ctx, body)
	if err != nil {
		return orderdemo.Operation{}, classifyTransport(ctx, err)
	}
	if response.JSON200 == nil {
		return orderdemo.Operation{}, classifyWriteResponse(response.StatusCode())
	}
	return mapOperation(*response.JSON200)
}

func (a *Adapter) RunBusinessProbe(ctx context.Context, _ requestcontext.Context, probeID string) (orderdemo.ProbeResult, error) {
	if strings.TrimSpace(probeID) == "" || len(probeID) > 100 {
		return orderdemo.ProbeResult{}, runtime.NewError(runtime.SchemaValidationFailed, "business probe ID is invalid", false)
	}
	response, err := a.readClient.RunOrderProcessingProbeWithResponse(ctx, generated.RunOrderProcessingProbeJSONRequestBody{ProbeId: probeID})
	if err != nil {
		return orderdemo.ProbeResult{}, classifyTransport(ctx, err)
	}
	if response.JSON200 == nil {
		return orderdemo.ProbeResult{}, classifyResponse(response.StatusCode())
	}
	value := response.JSON200
	if value.ProbeId != probeID || !value.Result.Valid() || value.DurationMs < 0 || value.DurationMs > 6000 {
		return orderdemo.ProbeResult{}, invalidResponse()
	}
	return orderdemo.ProbeResult{ProbeID: value.ProbeId, Result: string(value.Result), DurationMS: value.DurationMs, CompletedAt: value.CompletedAt}, nil
}

func mapOperation(value generated.OperationReceipt) (orderdemo.Operation, error) {
	before, okBefore := intValue(value.BeforeConcurrency)
	after, okAfter := intValue(value.AfterConcurrency)
	if !okBefore || before != 0 || !okAfter || after != 2 || value.OperationId == "" || value.InstanceEpoch == "" || value.BeforeVersion < 1 || value.AfterVersion <= value.BeforeVersion || !validDigest(value.IntentDigest) || value.ApprovalId == "" {
		return orderdemo.Operation{}, invalidResponse()
	}
	return orderdemo.Operation{OperationID: value.OperationId, InstanceEpoch: value.InstanceEpoch, BeforeVersion: value.BeforeVersion, AfterVersion: value.AfterVersion, BeforeConcurrency: before, AfterConcurrency: after, IntentDigest: value.IntentDigest, ApprovalID: value.ApprovalId, ExecutedAt: value.ExecutedAt}, nil
}

type boundedClient struct{ client *stdhttp.Client }

func (c *boundedClient) Do(request *stdhttp.Request) (*stdhttp.Response, error) {
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	contents, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	_ = response.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if len(contents) > maxResponseBytes {
		return nil, errResponseTooLarge
	}
	response.Body = io.NopCloser(bytes.NewReader(contents))
	return response, nil
}

func classifyTransport(ctx context.Context, err error) error {
	if errors.Is(err, errResponseTooLarge) {
		return runtime.NewError(runtime.SchemaValidationFailed, "order demo response exceeds the configured limit", false)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return runtime.NewError(runtime.ToolTimeout, "order demo request timed out", true)
	}
	return runtime.NewError(runtime.DependencyUnavailable, "order demo is unavailable", true)
}

func classifyResponse(status int) error {
	switch status {
	case stdhttp.StatusUnauthorized, stdhttp.StatusForbidden:
		return runtime.NewError(runtime.AdapterNotConfigured, "order demo operational credentials were rejected", false)
	case stdhttp.StatusNotFound:
		return runtime.NewError(runtime.ResourceNotFound, "order demo resource was not found", false)
	default:
		return runtime.NewError(runtime.DependencyUnavailable, "order demo returned an unsuccessful response", status >= 500 || status == stdhttp.StatusTooManyRequests)
	}
}

func classifyWriteResponse(status int) error {
	if status == stdhttp.StatusConflict {
		return runtime.NewError(runtime.TargetPreconditionFailed, "order demo remediation precondition failed", false)
	}
	return classifyResponse(status)
}

func invalidResponse() error {
	return runtime.NewError(runtime.SchemaValidationFailed, "order demo response is outside the operational contract", false)
}

func intValue(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), typed == float64(int(typed))
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func stringValue(value any) (string, bool) {
	result, ok := value.(string)
	return result, ok
}

func validConcurrency(value int) bool { return value >= 0 && value <= 4 }

func validStatus(status string) bool {
	switch status {
	case "queued", "processing", "completed", "failed":
		return true
	default:
		return false
	}
}

func validDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[7:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
