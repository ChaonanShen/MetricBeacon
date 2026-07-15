package bootstrap_test

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveIncidentMCPDiagnostic(t *testing.T) {
	if os.Getenv("MTB_RUN_LIVE_INCIDENT_MCP") != "1" {
		t.Skip("set MTB_RUN_LIVE_INCIDENT_MCP=1 to call a live incident-profile assistant-mcp")
	}
	endpoint := os.Getenv("MTB_LIVE_MCP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8081/mcp"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := newClient(t, endpoint, incidentHeaders(true))
	initialize(t, ctx, client)

	resolved := call(t, ctx, client, "playbook.resolve_alert", map[string]any{"sourceId": "demo-grafana", "alertName": "OrderQueueBacklog", "labels": map[string]string{"service_ref": "order-demo"}})
	if resolved.IsError {
		t.Fatal("live alert resolution failed")
	}
	var resolution struct{ PlaybookId, PlaybookVersion, PlaybookDigest, ServiceRef string }
	decodeStructured(t, resolved, &resolution)
	started := call(t, ctx, client, "playbook.start_run", map[string]any{"playbookId": resolution.PlaybookId, "version": resolution.PlaybookVersion, "digest": resolution.PlaybookDigest, "serviceRef": resolution.ServiceRef})
	if started.IsError {
		t.Fatal("live Playbook start failed")
	}
	var start struct{ Checkpoint, Status string }
	decodeStructured(t, started, &start)
	worker := call(t, ctx, client, "order_service.get_worker_state", map[string]any{})
	if worker.IsError {
		t.Fatal("live worker read failed")
	}
	var workerState struct{ ConfiguredConcurrency, EffectiveConcurrency, ActiveWorkers, Version int }
	decodeStructured(t, worker, &workerState)
	if workerState.ConfiguredConcurrency != 0 || workerState.EffectiveConcurrency != 0 || workerState.ActiveWorkers != 0 || workerState.Version < 2 {
		t.Fatalf("live worker is not stopped through the real Operational API: %#v", workerState)
	}
	resumed := call(t, ctx, client, "playbook.resume_run", map[string]any{"checkpoint": start.Checkpoint, "diagnosis": map[string]any{"primaryHypothesis": "worker concurrency is stopped", "evidenceRefs": []string{"order_service.get_worker_state", "order_service.get_worker_policy"}, "alternatives": []string{"slow processing", "dependency errors"}, "confidence": 0.95, "candidateAction": "restore_worker_concurrency"}})
	if resumed.IsError {
		t.Fatal("live Playbook resume failed")
	}
	var prepared struct {
		Status      string
		IntentDraft *struct {
			InstanceEpoch                                        string
			ExpectedVersion, BeforeConcurrency, AfterConcurrency int
		}
	}
	decodeStructured(t, resumed, &prepared)
	if prepared.Status != "needs_approval" || prepared.IntentDraft == nil || prepared.IntentDraft.InstanceEpoch == "" || prepared.IntentDraft.ExpectedVersion != workerState.Version || prepared.IntentDraft.BeforeConcurrency != 0 || prepared.IntentDraft.AfterConcurrency != 2 {
		t.Fatalf("live Intent is not bound to observed runtime/version: %#v", prepared)
	}
	t.Logf("live incident diagnosis prepared epoch=%s version=%d", prepared.IntentDraft.InstanceEpoch, prepared.IntentDraft.ExpectedVersion)
}
