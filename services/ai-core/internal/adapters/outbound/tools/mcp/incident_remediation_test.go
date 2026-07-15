package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	incidentport "mini-torchbearing.local/services/ai-core/internal/ports/incident"
)

func TestIncidentRemediationToolsetUsesOnlyTypedToolsAndForwardsRemediationIdentity(t *testing.T) {
	now := time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	receipt := `{"operationId":"operation-1","instanceEpoch":"epoch-1","beforeVersion":2,"afterVersion":3,"beforeConcurrency":0,"afterConcurrency":2,"intentDigest":"sha256:intent","approvalId":"approval-1","executedAt":"` + now + `"}`
	gateway := &incidentGatewayStub{responses: map[string]string{
		restoreWorkerTool:   receipt,
		getOperationTool:    receipt,
		getRuntimeTool:      `{"serviceRef":"order-demo","instanceEpoch":"epoch-1","supervisorStatus":"running","startedAt":"` + now + `"}`,
		getWorkerTool:       `{"serviceRef":"order-demo","instanceEpoch":"epoch-1","configuredConcurrency":2,"effectiveConcurrency":2,"activeWorkers":2,"inflightOrders":0,"version":3,"observedAt":"` + now + `"}`,
		recoveryMetricsTool: `{"windowSeconds":30,"acceptedDelta":2,"completedDelta":4,"queueDepth":1,"oldestAgeSeconds":0.2,"observedAt":"` + now + `"}`,
		businessProbeTool:   `{"probeId":"probe-1","result":"completed","durationMs":120,"completedAt":"` + now + `"}`,
	}}
	identity := incidentIdentity()
	identity.Permissions = []string{"incidents:remediate"}
	toolset := NewIncidentRemediationToolset(gateway)

	gotReceipt, writeEvidence, err := toolset.RestoreWorkerConcurrency(context.Background(), identity, incidentport.RestoreRequest{OperationID: "operation-1", InstanceEpoch: "epoch-1", ExpectedVersion: 2, IntentDigest: "sha256:intent", ApprovalID: "approval-1", ApprovalEvidence: "secret-evidence"})
	if err != nil || gotReceipt.AfterVersion != 3 || gotReceipt.AfterConcurrency != 2 {
		t.Fatalf("receipt=%#v err=%v", gotReceipt, err)
	}
	if _, _, err = toolset.GetOperation(context.Background(), identity, "operation-1"); err != nil {
		t.Fatal(err)
	}
	if value, _, err := toolset.GetRuntime(context.Background(), identity); err != nil || value.SupervisorStatus != "running" {
		t.Fatalf("runtime=%#v err=%v", value, err)
	}
	if value, _, err := toolset.GetWorker(context.Background(), identity); err != nil || value.Version != 3 {
		t.Fatalf("worker=%#v err=%v", value, err)
	}
	if value, _, err := toolset.GetRecoveryMetrics(context.Background(), identity); err != nil || value.WindowSeconds != 30 {
		t.Fatalf("metrics=%#v err=%v", value, err)
	}
	if value, _, err := toolset.RunBusinessProbe(context.Background(), identity, "probe-1"); err != nil || value.Result != "completed" {
		t.Fatalf("probe=%#v err=%v", value, err)
	}

	wantNames := []string{restoreWorkerTool, getOperationTool, getRuntimeTool, getWorkerTool, recoveryMetricsTool, businessProbeTool}
	if got := gateway.names(); !equalStrings(got, wantNames) {
		t.Fatalf("calls=%#v", got)
	}
	for _, call := range gateway.calls {
		if len(call.identity.Permissions) != 1 || call.identity.Permissions[0] != "incidents:remediate" {
			t.Fatalf("identity=%#v", call.identity)
		}
	}
	if !strings.Contains(string(gateway.calls[0].call.Arguments), "secret-evidence") {
		t.Fatal("approval evidence was not forwarded to the boundary")
	}
	if strings.Contains(string(writeEvidence.InputSummary), "secret-evidence") || strings.Contains(string(writeEvidence.OutputSummary), "secret-evidence") {
		t.Fatalf("approval evidence leaked into persistent summary: %#v", writeEvidence)
	}
	var summary map[string]any
	if err := json.Unmarshal(writeEvidence.InputSummary, &summary); err != nil || summary["approvalEvidencePresent"] != true {
		t.Fatalf("summary=%s err=%v", writeEvidence.InputSummary, err)
	}
}

func TestIncidentRemediationToolsetRejectsUnsafeOrMalformedCalls(t *testing.T) {
	tests := []struct {
		name string
		run  func(*IncidentRemediationToolset) error
	}{
		{"missing approval evidence", func(toolset *IncidentRemediationToolset) error {
			_, _, err := toolset.RestoreWorkerConcurrency(context.Background(), incidentIdentity(), incidentport.RestoreRequest{OperationID: "operation-1", InstanceEpoch: "epoch-1", ExpectedVersion: 2, IntentDigest: "digest", ApprovalID: "approval-1"})
			return err
		}},
		{"empty operation lookup", func(toolset *IncidentRemediationToolset) error {
			_, _, err := toolset.GetOperation(context.Background(), incidentIdentity(), "")
			return err
		}},
		{"empty probe id", func(toolset *IncidentRemediationToolset) error {
			_, _, err := toolset.RunBusinessProbe(context.Background(), incidentIdentity(), "")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gateway := &incidentGatewayStub{responses: map[string]string{}}
			if err := test.run(NewIncidentRemediationToolset(gateway)); err == nil {
				t.Fatal("expected rejection")
			}
			if len(gateway.calls) != 0 {
				t.Fatalf("unsafe boundary call=%#v", gateway.calls)
			}
		})
	}
}

func TestIncidentRemediationToolsetRejectsNonIntegralOrIncompleteResults(t *testing.T) {
	now := time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	tests := []struct {
		name, tool, response string
		run                  func(*IncidentRemediationToolset) error
	}{
		{"fractional receipt", getOperationTool, `{"operationId":"operation-1","instanceEpoch":"epoch-1","beforeVersion":2,"afterVersion":3,"beforeConcurrency":0.5,"afterConcurrency":2,"intentDigest":"digest","approvalId":"approval-1","executedAt":"` + now + `"}`, func(toolset *IncidentRemediationToolset) error {
			_, _, err := toolset.GetOperation(context.Background(), incidentIdentity(), "operation-1")
			return err
		}},
		{"missing runtime epoch", getRuntimeTool, `{"serviceRef":"order-demo","instanceEpoch":"","supervisorStatus":"running","startedAt":"` + now + `"}`, func(toolset *IncidentRemediationToolset) error {
			_, _, err := toolset.GetRuntime(context.Background(), incidentIdentity())
			return err
		}},
		{"missing probe timestamp", businessProbeTool, `{"probeId":"probe-1","result":"completed","durationMs":10}`, func(toolset *IncidentRemediationToolset) error {
			_, _, err := toolset.RunBusinessProbe(context.Background(), incidentIdentity(), "probe-1")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gateway := &incidentGatewayStub{responses: map[string]string{test.tool: test.response}}
			if err := test.run(NewIncidentRemediationToolset(gateway)); err == nil {
				t.Fatal("expected schema rejection")
			}
		})
	}
}
