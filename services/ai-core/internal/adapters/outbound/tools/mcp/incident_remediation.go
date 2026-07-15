package mcp

import (
	"context"
	"encoding/json"
	"time"

	generated "mini-torchbearing.local/packages/generated-contracts/go"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	incidentport "mini-torchbearing.local/services/ai-core/internal/ports/incident"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

const (
	restoreWorkerTool   = "order_service.restore_worker_concurrency"
	getOperationTool    = "order_service.get_operation"
	getRuntimeTool      = "order_service.get_runtime"
	getWorkerTool       = "order_service.get_worker_state"
	recoveryMetricsTool = "order_service.get_recovery_metrics"
	businessProbeTool   = "order_service.run_business_probe"
)

type IncidentRemediationToolset struct{ gateway tools.Gateway }

var _ incidentport.RemediationToolset = (*IncidentRemediationToolset)(nil)

func NewIncidentRemediationToolset(gateway tools.Gateway) *IncidentRemediationToolset {
	return &IncidentRemediationToolset{gateway: gateway}
}

func (t *IncidentRemediationToolset) RestoreWorkerConcurrency(ctx context.Context, identity requestcontext.Context, request incidentport.RestoreRequest) (incidentport.OperationReceipt, incidentport.ToolEvidence, error) {
	if t == nil || t.gateway == nil || request.OperationID == "" || request.InstanceEpoch == "" || request.ExpectedVersion < 1 || request.IntentDigest == "" || request.ApprovalID == "" || request.ApprovalEvidence == "" {
		return incidentport.OperationReceipt{}, incidentport.ToolEvidence{}, common.NewError(common.InvalidArgument, "remediation request is invalid", false)
	}
	input := generated.RestoreWorkerInput{OperationId: request.OperationID, InstanceEpoch: request.InstanceEpoch, ExpectedVersion: int(request.ExpectedVersion), ExpectedConcurrency: 0, NewConcurrency: 2, IntentDigest: request.IntentDigest, ApprovalId: request.ApprovalID, ApprovalEvidence: request.ApprovalEvidence}
	arguments, err := json.Marshal(input)
	if err != nil {
		return incidentport.OperationReceipt{}, incidentport.ToolEvidence{}, common.NewError(common.SchemaValidationFailed, "remediation input could not be encoded", false)
	}
	started := time.Now()
	result, err := t.gateway.CallTool(ctx, identity, tools.Call{Name: restoreWorkerTool, Version: "v1", Arguments: arguments})
	duration := time.Since(started).Milliseconds()
	inputSummary, _ := json.Marshal(map[string]any{"operationId": request.OperationID, "instanceEpoch": request.InstanceEpoch, "expectedVersion": request.ExpectedVersion, "expectedConcurrency": 0, "newConcurrency": 2, "intentDigest": request.IntentDigest, "approvalId": request.ApprovalID, "approvalEvidencePresent": true})
	if err != nil {
		return incidentport.OperationReceipt{}, incidentport.ToolEvidence{Name: restoreWorkerTool, InputSummary: inputSummary, DurationMS: duration}, err
	}
	var output generated.OperationOutput
	if err := json.Unmarshal(result.Content, &output); err != nil {
		return incidentport.OperationReceipt{}, incidentport.ToolEvidence{Name: restoreWorkerTool, InputSummary: inputSummary, DurationMS: duration}, common.NewError(common.SchemaValidationFailed, "remediation tool returned an invalid receipt", false)
	}
	receipt, err := operationReceipt(output)
	if err != nil {
		return incidentport.OperationReceipt{}, incidentport.ToolEvidence{Name: restoreWorkerTool, InputSummary: inputSummary, DurationMS: duration}, err
	}
	outputSummary, _ := json.Marshal(receipt)
	return receipt, incidentport.ToolEvidence{Name: restoreWorkerTool, InputSummary: inputSummary, OutputSummary: outputSummary, DurationMS: duration}, nil
}

func (t *IncidentRemediationToolset) GetOperation(ctx context.Context, identity requestcontext.Context, operationID string) (incidentport.OperationReceipt, incidentport.ToolEvidence, error) {
	if t == nil || t.gateway == nil || operationID == "" {
		return incidentport.OperationReceipt{}, incidentport.ToolEvidence{}, common.NewError(common.InvalidArgument, "operation lookup is invalid", false)
	}
	value, evidence, err := callTyped[generated.OperationOutput](ctx, t.gateway, identity, getOperationTool, generated.GetOperationInput{OperationId: operationID}, func(value generated.OperationOutput) any { return value })
	if err != nil {
		return incidentport.OperationReceipt{}, evidence, err
	}
	receipt, err := operationReceipt(value)
	return receipt, evidence, err
}

func (t *IncidentRemediationToolset) GetRuntime(ctx context.Context, identity requestcontext.Context) (incidentport.RuntimeState, incidentport.ToolEvidence, error) {
	if t == nil || t.gateway == nil {
		return incidentport.RuntimeState{}, incidentport.ToolEvidence{}, common.NewError(common.InvalidArgument, "remediation toolset is invalid", false)
	}
	value, evidence, err := callTyped[generated.RuntimeOutput](ctx, t.gateway, identity, getRuntimeTool, generated.EmptyInput{}, func(value generated.RuntimeOutput) any { return value })
	if err != nil {
		return incidentport.RuntimeState{}, evidence, err
	}
	serviceRef, ok := value.ServiceRef.(string)
	if !ok || serviceRef == "" || value.InstanceEpoch == "" || value.StartedAt.IsZero() {
		return incidentport.RuntimeState{}, evidence, invalidRemediationOutput()
	}
	return incidentport.RuntimeState{ServiceRef: serviceRef, InstanceEpoch: value.InstanceEpoch, SupervisorStatus: string(value.SupervisorStatus), StartedAt: value.StartedAt}, evidence, nil
}

func (t *IncidentRemediationToolset) GetWorker(ctx context.Context, identity requestcontext.Context) (incidentport.WorkerState, incidentport.ToolEvidence, error) {
	if t == nil || t.gateway == nil {
		return incidentport.WorkerState{}, incidentport.ToolEvidence{}, common.NewError(common.InvalidArgument, "remediation toolset is invalid", false)
	}
	value, evidence, err := callTyped[generated.WorkerOutput](ctx, t.gateway, identity, getWorkerTool, generated.EmptyInput{}, func(value generated.WorkerOutput) any { return value })
	if err != nil {
		return incidentport.WorkerState{}, evidence, err
	}
	serviceRef, serviceOK := value.ServiceRef.(string)
	configured, configuredOK := exactInt(value.ConfiguredConcurrency)
	effective, effectiveOK := exactInt(value.EffectiveConcurrency)
	version, versionOK := exactInt(value.Version)
	if !serviceOK || serviceRef == "" || value.InstanceEpoch == "" || !configuredOK || !effectiveOK || !versionOK || value.ObservedAt.IsZero() {
		return incidentport.WorkerState{}, evidence, invalidRemediationOutput()
	}
	return incidentport.WorkerState{ServiceRef: serviceRef, InstanceEpoch: value.InstanceEpoch, ConfiguredConcurrency: configured, EffectiveConcurrency: effective, ActiveWorkers: value.ActiveWorkers, InflightOrders: value.InflightOrders, Version: int64(version), ObservedAt: value.ObservedAt}, evidence, nil
}

func (t *IncidentRemediationToolset) GetRecoveryMetrics(ctx context.Context, identity requestcontext.Context) (incidentport.RecoveryMetrics, incidentport.ToolEvidence, error) {
	if t == nil || t.gateway == nil {
		return incidentport.RecoveryMetrics{}, incidentport.ToolEvidence{}, common.NewError(common.InvalidArgument, "remediation toolset is invalid", false)
	}
	value, evidence, err := callTyped[generated.RecoveryMetricsOutput](ctx, t.gateway, identity, recoveryMetricsTool, generated.EmptyInput{}, func(value generated.RecoveryMetricsOutput) any { return value })
	if err != nil {
		return incidentport.RecoveryMetrics{}, evidence, err
	}
	window, ok := exactInt(value.WindowSeconds)
	if !ok || value.ObservedAt.IsZero() {
		return incidentport.RecoveryMetrics{}, evidence, invalidRemediationOutput()
	}
	return incidentport.RecoveryMetrics{WindowSeconds: window, AcceptedDelta: float64(value.AcceptedDelta), CompletedDelta: float64(value.CompletedDelta), QueueDepth: float64(value.QueueDepth), OldestAgeSeconds: float64(value.OldestAgeSeconds), ObservedAt: value.ObservedAt}, evidence, nil
}

func (t *IncidentRemediationToolset) RunBusinessProbe(ctx context.Context, identity requestcontext.Context, probeID string) (incidentport.BusinessProbe, incidentport.ToolEvidence, error) {
	if t == nil || t.gateway == nil || probeID == "" {
		return incidentport.BusinessProbe{}, incidentport.ToolEvidence{}, common.NewError(common.InvalidArgument, "business probe request is invalid", false)
	}
	value, evidence, err := callTyped[generated.BusinessProbeOutput](ctx, t.gateway, identity, businessProbeTool, generated.BusinessProbeInput{ProbeId: probeID}, func(value generated.BusinessProbeOutput) any { return value })
	if err != nil {
		return incidentport.BusinessProbe{}, evidence, err
	}
	if value.ProbeId == "" || value.CompletedAt == nil {
		return incidentport.BusinessProbe{}, evidence, invalidRemediationOutput()
	}
	return incidentport.BusinessProbe{ProbeID: value.ProbeId, Result: string(value.Result), DurationMS: value.DurationMs, CompletedAt: *value.CompletedAt}, evidence, nil
}

func operationReceipt(value generated.OperationOutput) (incidentport.OperationReceipt, error) {
	before, beforeOK := exactInt(value.BeforeConcurrency)
	after, afterOK := exactInt(value.AfterConcurrency)
	if !beforeOK || !afterOK || value.OperationId == "" || value.InstanceEpoch == "" || value.IntentDigest == "" || value.ApprovalId == "" || value.ExecutedAt.IsZero() {
		return incidentport.OperationReceipt{}, invalidRemediationOutput()
	}
	return incidentport.OperationReceipt{OperationID: value.OperationId, InstanceEpoch: value.InstanceEpoch, BeforeVersion: int64(value.BeforeVersion), AfterVersion: int64(value.AfterVersion), BeforeConcurrency: before, AfterConcurrency: after, IntentDigest: value.IntentDigest, ApprovalID: value.ApprovalId, ExecutedAt: value.ExecutedAt}, nil
}

func invalidRemediationOutput() error {
	return common.NewError(common.SchemaValidationFailed, "remediation tool returned an invalid result", false)
}
