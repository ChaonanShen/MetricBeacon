package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	generated "mini-torchbearing.local/packages/generated-contracts/go"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	incidentport "mini-torchbearing.local/services/ai-core/internal/ports/incident"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

const (
	resolveAlertTool = "playbook.resolve_alert"
	startRunTool     = "playbook.start_run"
	resumeRunTool    = "playbook.resume_run"
	queueTool        = "order_service.get_queue_snapshot"
	workerTool       = "order_service.get_worker_state"
	policyTool       = "order_service.get_worker_policy"
	recentTool       = "order_service.get_recent_outcomes"
)

type IncidentToolset struct{ gateway tools.Gateway }

var _ incidentport.Toolset = (*IncidentToolset)(nil)

func NewIncidentToolset(gateway tools.Gateway) *IncidentToolset {
	return &IncidentToolset{gateway: gateway}
}

func (t *IncidentToolset) ResolveAndStart(ctx context.Context, identity requestcontext.Context, sourceID, alertName string, labels map[string]string) (incidentport.ResolvedRun, error) {
	if t == nil || t.gateway == nil || identity.TenantID == "" || identity.OrgID == "" || sourceID == "" || alertName == "" || len(labels) == 0 {
		return incidentport.ResolvedRun{}, common.NewError(common.InvalidArgument, "incident resolve request is invalid", false)
	}
	resolveInput := generated.ResolveAlertInput{SourceId: sourceID, AlertName: alertName, Labels: labels}
	resolved, resolveEvidence, err := callTyped[generated.ResolveAlertOutput](ctx, t.gateway, identity, resolveAlertTool, resolveInput, func(value generated.ResolveAlertOutput) any {
		return map[string]any{"mappingId": value.MappingId, "mappingDigest": value.MappingDigest, "playbookId": value.PlaybookId, "playbookVersion": value.PlaybookVersion, "playbookDigest": value.PlaybookDigest, "serviceRef": value.ServiceRef}
	})
	if err != nil {
		return incidentport.ResolvedRun{}, err
	}
	serviceRef, ok := resolved.ServiceRef.(string)
	if !ok || serviceRef == "" {
		return incidentport.ResolvedRun{}, common.NewError(common.SchemaValidationFailed, "resolved playbook service reference is invalid", false)
	}
	startInput := generated.StartRunInput{PlaybookId: resolved.PlaybookId, Version: resolved.PlaybookVersion, Digest: resolved.PlaybookDigest, ServiceRef: serviceRef}
	started, startEvidence, err := callTyped[generated.StartRunOutput](ctx, t.gateway, identity, startRunTool, startInput, func(value generated.StartRunOutput) any {
		return map[string]any{"status": value.Status, "checkpointPresent": value.Checkpoint != "", "assetCount": len(value.AssetRefs), "capabilityIds": value.CapabilityIds}
	})
	if err != nil {
		return incidentport.ResolvedRun{}, err
	}
	if fmt.Sprint(started.Status) != "needs_agent" || started.Checkpoint == "" {
		return incidentport.ResolvedRun{}, common.NewError(common.SchemaValidationFailed, "playbook did not pause for diagnosis", false)
	}
	assetRefs := make([]task.AssetRef, 0, len(started.AssetRefs)+1)
	assetRefs = append(assetRefs, task.AssetRef{Kind: "alert_mapping", ID: resolved.MappingId, Version: "1", Digest: prefixedDigest(resolved.MappingDigest)})
	for _, ref := range started.AssetRefs {
		assetRefs = append(assetRefs, task.AssetRef{Kind: string(ref.Kind), ID: ref.Id, Version: ref.Version, Digest: prefixedDigest(ref.Digest)})
	}
	return incidentport.ResolvedRun{MappingID: resolved.MappingId, MappingDigest: prefixedDigest(resolved.MappingDigest), PlaybookID: resolved.PlaybookId, PlaybookVersion: resolved.PlaybookVersion, PlaybookDigest: prefixedDigest(resolved.PlaybookDigest), ServiceRef: serviceRef, Checkpoint: started.Checkpoint, AssetRefs: assetRefs, Evidence: []incidentport.ToolEvidence{resolveEvidence, startEvidence}}, nil
}

func (t *IncidentToolset) Observe(ctx context.Context, identity requestcontext.Context, serviceRef string) (incidentport.Observation, error) {
	if t == nil || t.gateway == nil || serviceRef == "" {
		return incidentport.Observation{}, common.NewError(common.InvalidArgument, "incident observation request is invalid", false)
	}
	queue, queueEvidence, err := callTyped[generated.QueueOutput](ctx, t.gateway, identity, queueTool, generated.EmptyInput{}, func(value generated.QueueOutput) any { return value })
	if err != nil {
		return incidentport.Observation{}, err
	}
	worker, workerEvidence, err := callTyped[generated.WorkerOutput](ctx, t.gateway, identity, workerTool, generated.EmptyInput{}, func(value generated.WorkerOutput) any { return value })
	if err != nil {
		return incidentport.Observation{}, err
	}
	policy, policyEvidence, err := callTyped[generated.PolicyOutput](ctx, t.gateway, identity, policyTool, generated.EmptyInput{}, func(value generated.PolicyOutput) any { return value })
	if err != nil {
		return incidentport.Observation{}, err
	}
	limit := 10
	recent, recentEvidence, err := callTyped[generated.RecentOutcomesOutput](ctx, t.gateway, identity, recentTool, generated.RecentOutcomesInput{Limit: &limit}, func(value generated.RecentOutcomesOutput) any {
		counts := map[string]int{}
		for _, order := range value.Orders {
			counts[string(order.Status)]++
			if order.FailureReason != nil {
				counts["reason:"+string(*order.FailureReason)]++
			}
		}
		return map[string]any{"counts": counts}
	})
	if err != nil {
		return incidentport.Observation{}, err
	}
	diagnosis := classifyObservation(serviceRef, queue, worker, policy, recent)
	return incidentport.Observation{Diagnosis: diagnosis, Evidence: []incidentport.ToolEvidence{queueEvidence, workerEvidence, policyEvidence, recentEvidence}}, nil
}

func (t *IncidentToolset) Prepare(ctx context.Context, identity requestcontext.Context, checkpoint string, diagnosis task.Diagnosis) (incidentport.PreparedRun, error) {
	if t == nil || t.gateway == nil || checkpoint == "" {
		return incidentport.PreparedRun{}, common.NewError(common.InvalidArgument, "incident prepare request is invalid", false)
	}
	input := generated.ResumeRunInput{Checkpoint: checkpoint}
	input.Diagnosis.PrimaryHypothesis = diagnosis.PrimaryHypothesis
	input.Diagnosis.EvidenceRefs = append([]string(nil), diagnosis.EvidenceRefs...)
	input.Diagnosis.Alternatives = append([]string(nil), diagnosis.AlternativeHypotheses...)
	input.Diagnosis.Confidence = float32(diagnosis.Confidence)
	input.Diagnosis.CandidateAction = generated.ResumeRunInputDiagnosisCandidateAction(diagnosis.CandidateAction)
	value, _, err := callTyped[generated.ResumeRunOutput](ctx, t.gateway, identity, resumeRunTool, input, func(value generated.ResumeRunOutput) any {
		return map[string]any{"status": value.Status, "checkpointPresent": value.Checkpoint != "", "intentPresent": value.IntentDraft != nil}
	})
	if err != nil {
		return incidentport.PreparedRun{}, err
	}
	result := incidentport.PreparedRun{Status: string(value.Status), Checkpoint: value.Checkpoint}
	if result.Checkpoint == "" || (result.Status != "needs_approval" && result.Status != "completed") || (result.Status == "completed" && value.IntentDraft != nil) || (result.Status == "needs_approval" && value.IntentDraft == nil) {
		return incidentport.PreparedRun{}, common.NewError(common.SchemaValidationFailed, "playbook prepare result is invalid", false)
	}
	if value.IntentDraft != nil {
		capabilityID, capabilityOK := value.IntentDraft.CapabilityId.(string)
		serviceRef, serviceOK := value.IntentDraft.ServiceRef.(string)
		before, beforeOK := exactInt(value.IntentDraft.BeforeConcurrency)
		after, afterOK := exactInt(value.IntentDraft.AfterConcurrency)
		if !capabilityOK || !serviceOK || !beforeOK || !afterOK {
			return incidentport.PreparedRun{}, common.NewError(common.SchemaValidationFailed, "playbook Intent draft is invalid", false)
		}
		result.Intent = &incidentport.PreparedIntent{CapabilityID: capabilityID, ServiceRef: serviceRef, InstanceEpoch: value.IntentDraft.InstanceEpoch, ExpectedVersion: int64(value.IntentDraft.ExpectedVersion), ObservedAt: value.IntentDraft.ObservedAt, PolicyDigest: value.IntentDraft.PolicyDigest, PlaybookDigest: value.IntentDraft.PlaybookDigest, BeforeConcurrency: before, AfterConcurrency: after, RiskSummary: value.IntentDraft.Risk}
	}
	return result, nil
}

func callTyped[T any](ctx context.Context, gateway tools.Gateway, identity requestcontext.Context, name string, input any, summarize func(T) any) (T, incidentport.ToolEvidence, error) {
	var zero T
	if gateway == nil || name == "" {
		return zero, incidentport.ToolEvidence{}, common.NewError(common.InvalidArgument, "incident tool call is invalid", false)
	}
	arguments, err := json.Marshal(input)
	if err != nil {
		return zero, incidentport.ToolEvidence{}, common.NewError(common.SchemaValidationFailed, "incident tool input could not be encoded", false)
	}
	started := time.Now()
	result, err := gateway.CallTool(ctx, identity, tools.Call{Name: name, Version: "v1", Arguments: arguments})
	duration := time.Since(started).Milliseconds()
	inputSummary := append(json.RawMessage(nil), arguments...)
	if err != nil {
		return zero, incidentport.ToolEvidence{Name: name, InputSummary: inputSummary, DurationMS: duration}, err
	}
	var output T
	if err := json.Unmarshal(result.Content, &output); err != nil {
		return zero, incidentport.ToolEvidence{}, common.NewError(common.SchemaValidationFailed, "incident tool returned an invalid result", false)
	}
	outputSummary, _ := json.Marshal(summarize(output))
	return output, incidentport.ToolEvidence{Name: name, InputSummary: inputSummary, OutputSummary: outputSummary, DurationMS: duration}, nil
}

func classifyObservation(serviceRef string, queue generated.QueueOutput, worker generated.WorkerOutput, policy generated.PolicyOutput, recent generated.RecentOutcomesOutput) task.Diagnosis {
	evidence := []string{queueTool, workerTool, policyTool, recentTool}
	workerService, workerOK := worker.ServiceRef.(string)
	policyService, policyOK := policy.ServiceRef.(string)
	expected, expectedOK := exactInt(policy.ExpectedConcurrency)
	fresh := math.Abs(queue.ObservedAt.Sub(worker.ObservedAt).Seconds()) <= 60
	consistent := workerOK && policyOK && expectedOK && workerService == serviceRef && policyService == serviceRef && worker.InstanceEpoch != "" && fresh
	if consistent && queue.Depth > 0 && worker.ConfiguredConcurrency == 0 && worker.EffectiveConcurrency == 0 && worker.ActiveWorkers == 0 && expected == 2 {
		return task.Diagnosis{PrimaryHypothesis: "worker_stopped", EvidenceRefs: evidence, AlternativeHypotheses: []string{"slow_processing", "dependency_errors"}, Confidence: 0.99, CandidateAction: "restore_worker_concurrency"}
	}
	hasDependencyFailure := false
	for _, order := range recent.Orders {
		if order.FailureReason != nil {
			hasDependencyFailure = true
		}
	}
	if consistent && worker.ConfiguredConcurrency == expected && hasDependencyFailure {
		return task.Diagnosis{PrimaryHypothesis: "dependency_errors", EvidenceRefs: evidence, AlternativeHypotheses: []string{"slow_processing"}, Confidence: 0.9, CandidateAction: "no_action"}
	}
	if consistent && worker.ConfiguredConcurrency == expected && worker.ActiveWorkers > 0 && queue.Depth > 0 && queue.OldestAgeSeconds >= 1 {
		return task.Diagnosis{PrimaryHypothesis: "slow_processing", EvidenceRefs: evidence, AlternativeHypotheses: []string{"dependency_errors"}, Confidence: 0.85, CandidateAction: "no_action"}
	}
	if consistent && worker.ConfiguredConcurrency == expected && queue.Depth == 0 {
		return task.Diagnosis{PrimaryHypothesis: "healthy", EvidenceRefs: evidence, Confidence: 0.95, CandidateAction: "no_action"}
	}
	return task.Diagnosis{PrimaryHypothesis: "insufficient_evidence", EvidenceRefs: evidence, AlternativeHypotheses: []string{"worker_stopped", "slow_processing", "dependency_errors"}, Confidence: 0.2, CandidateAction: "no_action"}
}

func exactInt(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		converted := int(number)
		return converted, float64(converted) == number
	case int:
		return number, true
	default:
		return 0, false
	}
}

func prefixedDigest(value string) string {
	if len(value) > 7 && value[:7] == "sha256:" {
		return value
	}
	return "sha256:" + value
}
