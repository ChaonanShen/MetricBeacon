package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

func TestIncidentToolsetResolvesAndPinsRun(t *testing.T) {
	gateway := &incidentGatewayStub{responses: map[string]string{
		resolveAlertTool: `{"mappingId":"order-demo","mappingDigest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","playbookId":"order-queue-backlog","playbookVersion":"1","playbookDigest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","serviceRef":"order-demo"}`,
		startRunTool:     `{"status":"needs_agent","checkpoint":"signed-checkpoint","assetRefs":[{"kind":"playbook","id":"order-queue-backlog","version":"1","digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},{"kind":"knowledge","id":"order-service","version":"1","digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},{"kind":"skill","id":"diagnose-order-backlog","version":"1","digest":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}],"capabilityIds":["order_service.get_worker_state"]}`,
	}}
	identity := incidentIdentity()
	result, err := NewIncidentToolset(gateway).ResolveAndStart(context.Background(), identity, "demo-grafana", "OrderQueueBacklog", map[string]string{"service_ref": "order-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ServiceRef != "order-demo" || result.Checkpoint != "signed-checkpoint" || len(result.AssetRefs) != 4 || result.AssetRefs[0].Kind != "alert_mapping" || result.PlaybookDigest != "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("resolved run = %#v", result)
	}
	if got := gateway.names(); !equalStrings(got, []string{resolveAlertTool, startRunTool}) {
		t.Fatalf("calls = %#v", got)
	}
	for _, call := range gateway.calls {
		if call.identity.TenantID != identity.TenantID || call.identity.OrgID != identity.OrgID || len(call.identity.Permissions) != 1 || call.identity.Permissions[0] != "incidents:diagnose" {
			t.Fatalf("identity was not forwarded: %#v", call.identity)
		}
	}
}

func TestIncidentToolsetClassifiesSimilarSymptomsWithoutWriteTools(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	tests := []struct {
		name, queue, worker, recent, wantHypothesis, wantAction string
	}{
		{"stopped", `{"depth":12,"capacity":100,"oldestAgeSeconds":8,"observedAt":"` + now + `"}`, `{"serviceRef":"order-demo","instanceEpoch":"epoch-1","configuredConcurrency":0,"effectiveConcurrency":0,"activeWorkers":0,"inflightOrders":0,"version":2,"observedAt":"` + now + `"}`, `{"orders":[]}`, "worker_stopped", "restore_worker_concurrency"},
		{"slow", `{"depth":6,"capacity":100,"oldestAgeSeconds":3,"observedAt":"` + now + `"}`, `{"serviceRef":"order-demo","instanceEpoch":"epoch-1","configuredConcurrency":2,"effectiveConcurrency":2,"activeWorkers":2,"inflightOrders":2,"version":1,"observedAt":"` + now + `"}`, `{"orders":[]}`, "slow_processing", "no_action"},
		{"dependency", `{"depth":4,"capacity":100,"oldestAgeSeconds":2,"observedAt":"` + now + `"}`, `{"serviceRef":"order-demo","instanceEpoch":"epoch-1","configuredConcurrency":2,"effectiveConcurrency":2,"activeWorkers":2,"inflightOrders":1,"version":1,"observedAt":"` + now + `"}`, `{"orders":[{"id":"redacted-by-summary","status":"failed","createdAt":"` + now + `","updatedAt":"` + now + `","failureReason":"dependency_unavailable"}]}`, "dependency_errors", "no_action"},
		{"healthy", `{"depth":0,"capacity":100,"oldestAgeSeconds":0,"observedAt":"` + now + `"}`, `{"serviceRef":"order-demo","instanceEpoch":"epoch-1","configuredConcurrency":2,"effectiveConcurrency":2,"activeWorkers":2,"inflightOrders":0,"version":1,"observedAt":"` + now + `"}`, `{"orders":[]}`, "healthy", "no_action"},
		{"inconsistent", `{"depth":12,"capacity":100,"oldestAgeSeconds":8,"observedAt":"` + now + `"}`, `{"serviceRef":"other-service","instanceEpoch":"epoch-1","configuredConcurrency":0,"effectiveConcurrency":0,"activeWorkers":0,"inflightOrders":0,"version":2,"observedAt":"` + now + `"}`, `{"orders":[]}`, "insufficient_evidence", "no_action"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gateway := &incidentGatewayStub{responses: map[string]string{
				queueTool: test.queue, workerTool: test.worker,
				policyTool: `{"serviceRef":"order-demo","expectedConcurrency":2,"minConcurrency":1,"maxConcurrency":4,"version":"v1","digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
				recentTool: test.recent,
			}}
			result, err := NewIncidentToolset(gateway).Observe(context.Background(), incidentIdentity(), "order-demo")
			if err != nil {
				t.Fatal(err)
			}
			if result.Diagnosis.PrimaryHypothesis != test.wantHypothesis || result.Diagnosis.CandidateAction != test.wantAction || len(result.Evidence) != 4 {
				t.Fatalf("diagnosis = %#v", result.Diagnosis)
			}
			if got := gateway.names(); !equalStrings(got, []string{queueTool, workerTool, policyTool, recentTool}) {
				t.Fatalf("calls = %#v", got)
			}
			for _, evidence := range result.Evidence {
				if evidence.Name == "order_service.get_operation" || evidence.Name == "order_service.restore_worker_concurrency" || evidence.Name == "fault.inject" || !json.Valid(evidence.OutputSummary) {
					t.Fatalf("unsafe evidence = %#v", evidence)
				}
			}
		})
	}
}

func TestIncidentToolsetRejectsMalformedToolResult(t *testing.T) {
	gateway := &incidentGatewayStub{responses: map[string]string{queueTool: `{not-json}`}}
	_, err := NewIncidentToolset(gateway).Observe(context.Background(), incidentIdentity(), "order-demo")
	var domainErr *common.DomainError
	if err == nil || !asDomainError(err, &domainErr) || domainErr.Code != common.SchemaValidationFailed {
		t.Fatalf("error = %v", err)
	}
}

func TestTypedIncidentCallReturnsSafeEvidenceWhenBoundaryFails(t *testing.T) {
	gateway := &incidentGatewayStub{err: common.NewError(common.ToolTimeout, "tool timed out", true)}
	_, evidence, err := callTyped[map[string]any](context.Background(), gateway, incidentIdentity(), "order_service.get_runtime", map[string]any{}, func(value map[string]any) any { return value })
	if err == nil || evidence.Name != "order_service.get_runtime" || !json.Valid(evidence.InputSummary) || len(evidence.OutputSummary) != 0 || evidence.DurationMS < 0 {
		t.Fatalf("evidence=%#v err=%v", evidence, err)
	}
}

func TestIncidentToolsetPreparesOnlyVersionBoundIntent(t *testing.T) {
	observedAt := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	gateway := &incidentGatewayStub{responses: map[string]string{
		resumeRunTool: `{"status":"needs_approval","checkpoint":"prepared-checkpoint","intentDraft":{"capabilityId":"order_service.restore_worker_concurrency","serviceRef":"order-demo","instanceEpoch":"epoch-1","expectedVersion":2,"observedAt":"` + observedAt.Format(time.RFC3339Nano) + `","policyDigest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","playbookDigest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","beforeConcurrency":0,"afterConcurrency":2,"risk":"bounded restore"}}`,
	}}
	diagnosis := task.Diagnosis{PrimaryHypothesis: "worker_stopped", EvidenceRefs: []string{workerTool, policyTool}, AlternativeHypotheses: []string{"slow_processing"}, Confidence: 0.99, CandidateAction: "restore_worker_concurrency"}
	result, err := NewIncidentToolset(gateway).Prepare(context.Background(), incidentIdentity(), "signed-checkpoint", diagnosis)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "needs_approval" || result.Checkpoint != "prepared-checkpoint" || result.Intent == nil || result.Intent.InstanceEpoch != "epoch-1" || result.Intent.ExpectedVersion != 2 || result.Intent.BeforeConcurrency != 0 || result.Intent.AfterConcurrency != 2 {
		t.Fatalf("prepared=%#v", result)
	}
	if got := gateway.names(); !equalStrings(got, []string{resumeRunTool}) {
		t.Fatalf("calls=%#v", got)
	}
	var input map[string]any
	if err := json.Unmarshal(gateway.calls[0].call.Arguments, &input); err != nil {
		t.Fatal(err)
	}
	if input["approvalEvidence"] != nil || input["operationId"] != nil || input["checkpoint"] != "signed-checkpoint" {
		t.Fatalf("unsafe prepare input=%#v", input)
	}
}

func TestIncidentToolsetAcceptsNoActionPrepareWithoutIntent(t *testing.T) {
	gateway := &incidentGatewayStub{responses: map[string]string{resumeRunTool: `{"status":"completed","checkpoint":"completed-checkpoint"}`}}
	result, err := NewIncidentToolset(gateway).Prepare(context.Background(), incidentIdentity(), "signed-checkpoint", task.Diagnosis{PrimaryHypothesis: "slow_processing", EvidenceRefs: []string{queueTool}, Confidence: 0.9, CandidateAction: "no_action"})
	if err != nil || result.Status != "completed" || result.Intent != nil {
		t.Fatalf("prepared=%#v err=%v", result, err)
	}
}

type incidentGatewayCall struct {
	identity requestcontext.Context
	call     tools.Call
}

type incidentGatewayStub struct {
	responses map[string]string
	calls     []incidentGatewayCall
	err       error
}

func (g *incidentGatewayStub) ListTools(context.Context, requestcontext.Context, tools.Filter) ([]tools.Descriptor, error) {
	return nil, nil
}
func (g *incidentGatewayStub) CallTool(_ context.Context, identity requestcontext.Context, call tools.Call) (tools.Result, error) {
	g.calls = append(g.calls, incidentGatewayCall{identity: identity, call: call})
	if g.err != nil {
		return tools.Result{}, g.err
	}
	return tools.Result{Content: []byte(g.responses[call.Name])}, nil
}
func (g *incidentGatewayStub) names() []string {
	values := make([]string, 0, len(g.calls))
	for _, call := range g.calls {
		values = append(values, call.call.Name)
	}
	return values
}

func incidentIdentity() requestcontext.Context {
	return requestcontext.Context{TenantID: "org:1", OrgID: "1", UserID: "system:grafana", Roles: []string{"IncidentAgent"}, Permissions: []string{"incidents:diagnose"}, RequestID: "request-1", TraceID: "trace-1"}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func asDomainError(err error, target **common.DomainError) bool {
	value, ok := err.(*common.DomainError)
	if ok {
		*target = value
	}
	return ok
}
