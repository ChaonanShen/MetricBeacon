package incident

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	approvalevidence "mini-torchbearing.local/packages/approval-evidence-go"
	generated "mini-torchbearing.local/packages/generated-contracts/go"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/playbook"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/assets"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/executionaudit"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/incidentmetrics"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/orderdemo"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

type Service struct {
	assets      assets.Port
	orders      orderdemo.Port
	engine      *playbook.Engine
	remediation orderdemo.RemediationPort
	metrics     incidentmetrics.Port
	evidence    *approvalevidence.Codec
	audit       executionaudit.Port
	now         func() time.Time
}

func NewService(assetPort assets.Port, orderPort orderdemo.Port, engine *playbook.Engine, remediation orderdemo.RemediationPort, metrics incidentmetrics.Port, evidence *approvalevidence.Codec, audit executionaudit.Port, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{assets: assetPort, orders: orderPort, engine: engine, remediation: remediation, metrics: metrics, evidence: evidence, audit: audit, now: now}
}

func (s *Service) KnowledgeGet(identity requestcontext.Context, input generated.AssetGetInput) (generated.AssetOutput, error) {
	if err := authorize(identity); err != nil {
		return generated.AssetOutput{}, err
	}
	value, err := s.assets.Knowledge(input.Id, input.Version)
	if err != nil {
		return generated.AssetOutput{}, err
	}
	return generated.AssetOutput{Id: value.ID, Version: value.Version, Digest: value.Digest, Content: value.Content}, nil
}

func (s *Service) SkillGet(identity requestcontext.Context, input generated.AssetGetInput) (generated.AssetOutput, error) {
	if err := authorize(identity); err != nil {
		return generated.AssetOutput{}, err
	}
	value, err := s.assets.Skill(input.Id, input.Version)
	if err != nil {
		return generated.AssetOutput{}, err
	}
	return generated.AssetOutput{Id: value.ID, Version: value.Version, Digest: value.Digest, Content: value.Content}, nil
}

func (s *Service) ResolveAlert(identity requestcontext.Context, input generated.ResolveAlertInput) (generated.ResolveAlertOutput, error) {
	if err := authorize(identity); err != nil {
		return generated.ResolveAlertOutput{}, err
	}
	value, err := s.assets.Resolve(assets.Alert{SourceID: input.SourceId, AlertName: input.AlertName, Labels: input.Labels})
	if err != nil {
		return generated.ResolveAlertOutput{}, err
	}
	return generated.ResolveAlertOutput{MappingId: value.MappingID, MappingDigest: value.MappingDigest, PlaybookId: value.PlaybookID, PlaybookVersion: value.PlaybookVersion, PlaybookDigest: value.PlaybookDigest, ServiceRef: value.ServiceRef}, nil
}

func (s *Service) StartRun(ctx context.Context, identity requestcontext.Context, input generated.StartRunInput) (generated.StartRunOutput, error) {
	if err := authorize(identity); err != nil {
		return generated.StartRunOutput{}, err
	}
	serviceRef, ok := input.ServiceRef.(string)
	if !ok {
		return generated.StartRunOutput{}, invalidInput()
	}
	value, err := s.engine.Start(ctx, identity, input.PlaybookId, input.Version, input.Digest, serviceRef)
	if err != nil {
		return generated.StartRunOutput{}, err
	}
	output := generated.StartRunOutput{Status: value.Status, Checkpoint: value.Checkpoint, CapabilityIds: value.CapabilityIDs, AssetRefs: make([]generated.AssetRef, 0, len(value.AssetRefs))}
	for _, reference := range value.AssetRefs {
		output.AssetRefs = append(output.AssetRefs, generated.AssetRef{Kind: generated.AssetRefKind(reference.Kind), Id: reference.ID, Version: reference.Version, Digest: reference.Digest})
	}
	return output, nil
}

func (s *Service) ResumeRun(ctx context.Context, identity requestcontext.Context, input generated.ResumeRunInput) (generated.ResumeRunOutput, error) {
	if err := authorize(identity); err != nil {
		return generated.ResumeRunOutput{}, err
	}
	value, err := s.engine.Resume(ctx, identity, input.Checkpoint, playbook.Diagnosis{PrimaryHypothesis: input.Diagnosis.PrimaryHypothesis, EvidenceRefs: input.Diagnosis.EvidenceRefs, Alternatives: input.Diagnosis.Alternatives, Confidence: float64(input.Diagnosis.Confidence), CandidateAction: string(input.Diagnosis.CandidateAction)})
	if err != nil {
		return generated.ResumeRunOutput{}, err
	}
	output := generated.ResumeRunOutput{Status: generated.ResumeRunOutputStatus(value.Status), Checkpoint: value.Checkpoint}
	if value.IntentDraft != nil {
		intent := value.IntentDraft
		output.IntentDraft = &generated.IntentDraft{CapabilityId: intent.CapabilityID, ServiceRef: intent.ServiceRef, InstanceEpoch: intent.InstanceEpoch, ExpectedVersion: intent.ExpectedVersion, ObservedAt: intent.ObservedAt, PolicyDigest: intent.PolicyDigest, PlaybookDigest: intent.PlaybookDigest, BeforeConcurrency: intent.BeforeConcurrency, AfterConcurrency: intent.AfterConcurrency, Risk: intent.Risk}
	}
	return output, nil
}

func (s *Service) GetRuntime(ctx context.Context, identity requestcontext.Context) (generated.RuntimeOutput, error) {
	if err := authorizeReadOrRemediate(identity); err != nil {
		return generated.RuntimeOutput{}, err
	}
	value, err := s.orders.GetRuntime(ctx, identity)
	if err != nil {
		return generated.RuntimeOutput{}, err
	}
	return generated.RuntimeOutput{ServiceRef: value.ServiceRef, InstanceEpoch: value.InstanceEpoch, StartedAt: value.StartedAt, SupervisorStatus: generated.RuntimeOutputSupervisorStatus(value.SupervisorStatus)}, nil
}

func (s *Service) GetQueue(ctx context.Context, identity requestcontext.Context) (generated.QueueOutput, error) {
	if err := authorize(identity); err != nil {
		return generated.QueueOutput{}, err
	}
	value, err := s.orders.GetQueue(ctx, identity)
	if err != nil {
		return generated.QueueOutput{}, err
	}
	return generated.QueueOutput{Depth: value.Depth, Capacity: value.Capacity, OldestAgeSeconds: float32(value.OldestAgeSeconds), ObservedAt: value.ObservedAt}, nil
}

func (s *Service) GetWorker(ctx context.Context, identity requestcontext.Context) (generated.WorkerOutput, error) {
	if err := authorizeReadOrRemediate(identity); err != nil {
		return generated.WorkerOutput{}, err
	}
	value, err := s.orders.GetWorker(ctx, identity)
	if err != nil {
		return generated.WorkerOutput{}, err
	}
	return generated.WorkerOutput{ServiceRef: value.ServiceRef, InstanceEpoch: value.InstanceEpoch, ConfiguredConcurrency: value.ConfiguredConcurrency, EffectiveConcurrency: value.EffectiveConcurrency, ActiveWorkers: value.ActiveWorkers, InflightOrders: value.InflightOrders, Version: value.Version, ObservedAt: value.ObservedAt}, nil
}

func (s *Service) GetPolicy(ctx context.Context, identity requestcontext.Context) (generated.PolicyOutput, error) {
	if err := authorize(identity); err != nil {
		return generated.PolicyOutput{}, err
	}
	value, err := s.orders.GetPolicy(ctx, identity)
	if err != nil {
		return generated.PolicyOutput{}, err
	}
	return generated.PolicyOutput{ServiceRef: value.ServiceRef, ExpectedConcurrency: value.ExpectedConcurrency, MinConcurrency: value.MinConcurrency, MaxConcurrency: value.MaxConcurrency, Version: value.Version, Digest: value.Digest}, nil
}

func (s *Service) GetRecentOutcomes(ctx context.Context, identity requestcontext.Context, input generated.RecentOutcomesInput) (generated.RecentOutcomesOutput, error) {
	if err := authorize(identity); err != nil {
		return generated.RecentOutcomesOutput{}, err
	}
	limit := 10
	if input.Limit != nil {
		limit = *input.Limit
	}
	status := ""
	if input.Status != nil {
		status = string(*input.Status)
	}
	value, err := s.orders.GetRecentOutcomes(ctx, identity, orderdemo.RecentRequest{Status: status, Limit: limit})
	if err != nil {
		return generated.RecentOutcomesOutput{}, err
	}
	var output generated.RecentOutcomesOutput
	if err := mapValue(struct {
		Orders []orderdemo.OrderOutcome `json:"orders"`
	}{Orders: value}, &output); err != nil {
		return generated.RecentOutcomesOutput{}, err
	}
	return output, nil
}

func (s *Service) GetOperation(ctx context.Context, identity requestcontext.Context, input generated.GetOperationInput) (generated.OperationOutput, error) {
	if err := authorizeReadOrRemediate(identity); err != nil {
		return generated.OperationOutput{}, err
	}
	value, err := s.orders.GetOperation(ctx, identity, input.OperationId)
	if err != nil {
		return generated.OperationOutput{}, err
	}
	return generated.OperationOutput{OperationId: value.OperationID, InstanceEpoch: value.InstanceEpoch, BeforeVersion: value.BeforeVersion, AfterVersion: value.AfterVersion, BeforeConcurrency: value.BeforeConcurrency, AfterConcurrency: value.AfterConcurrency, IntentDigest: value.IntentDigest, ApprovalId: value.ApprovalID, ExecutedAt: value.ExecutedAt}, nil
}

func (s *Service) RestoreWorkerConcurrency(ctx context.Context, identity requestcontext.Context, input generated.RestoreWorkerInput) (generated.OperationOutput, error) {
	if err := runtime.RequirePermission(identity, runtime.PermissionIncidentRemediate); err != nil {
		return generated.OperationOutput{}, err
	}
	if s.remediation == nil || s.evidence == nil || s.audit == nil {
		return generated.OperationOutput{}, runtime.NewError(runtime.AdapterNotConfigured, "Incident remediation is not configured", false)
	}
	expectedConcurrency, expectedOK := exactInteger(input.ExpectedConcurrency)
	newConcurrency, newOK := exactInteger(input.NewConcurrency)
	if !expectedOK || expectedConcurrency != 0 || !newOK || newConcurrency != 2 {
		return generated.OperationOutput{}, invalidInput()
	}
	claims, err := s.evidence.Verify(input.ApprovalEvidence, approvalevidence.ExpectedScope{TenantID: identity.TenantID, OrgID: identity.OrgID, ApprovalID: input.ApprovalId, IntentDigest: input.IntentDigest, CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: "order-demo", InstanceEpoch: input.InstanceEpoch, ExpectedVersion: input.ExpectedVersion, OperationID: input.OperationId}, s.now())
	if err != nil {
		code := runtime.ApprovalRequired
		if errors.Is(err, approvalevidence.ErrExpired) {
			code = runtime.ApprovalExpired
		}
		return generated.OperationOutput{}, runtime.NewError(code, "valid ApprovalEvidence is required", false)
	}
	record := executionaudit.Record{ID: input.OperationId + ":authorized", TenantID: claims.TenantID, OrgID: claims.OrgID, TaskID: claims.TaskID, ApprovalID: claims.ApprovalID, IntentDigest: claims.IntentDigest, OperationID: claims.OperationID, Phase: "execute", Outcome: "authorized", OccurredAt: s.now()}
	if err := s.audit.Append(ctx, record); err != nil {
		return generated.OperationOutput{}, runtime.NewError(runtime.DependencyUnavailable, "execution audit is unavailable", true)
	}
	value, err := s.remediation.RestoreWorkerConcurrency(ctx, identity, orderdemo.RemediationRequest{OperationID: input.OperationId, InstanceEpoch: input.InstanceEpoch, ExpectedVersion: input.ExpectedVersion, ExpectedConcurrency: expectedConcurrency, NewConcurrency: newConcurrency, IntentDigest: input.IntentDigest, ApprovalID: input.ApprovalId})
	if err != nil {
		record.ID, record.Outcome, record.OccurredAt = input.OperationId+":failed", "failed", s.now()
		_ = s.audit.Append(context.Background(), record)
		return generated.OperationOutput{}, err
	}
	record.ID, record.Outcome, record.OccurredAt = input.OperationId+":succeeded", "succeeded", s.now()
	if err := s.audit.Append(ctx, record); err != nil {
		return generated.OperationOutput{}, runtime.NewError(runtime.DependencyUnavailable, "execution result audit is unavailable; reconcile the operation", true)
	}
	return operationOutput(value), nil
}

func exactInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), typed == float64(int(typed))
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func (s *Service) GetRecoveryMetrics(ctx context.Context, identity requestcontext.Context) (generated.RecoveryMetricsOutput, error) {
	if err := runtime.RequirePermission(identity, runtime.PermissionIncidentRemediate); err != nil {
		return generated.RecoveryMetricsOutput{}, err
	}
	if s.metrics == nil {
		return generated.RecoveryMetricsOutput{}, runtime.NewError(runtime.AdapterNotConfigured, "Incident metrics are not configured", false)
	}
	value, err := s.metrics.GetRecovery(ctx, identity)
	if err != nil {
		return generated.RecoveryMetricsOutput{}, err
	}
	return generated.RecoveryMetricsOutput{WindowSeconds: value.WindowSeconds, AcceptedDelta: float32(value.AcceptedDelta), CompletedDelta: float32(value.CompletedDelta), QueueDepth: float32(value.QueueDepth), OldestAgeSeconds: float32(value.OldestAgeSeconds), ObservedAt: value.ObservedAt}, nil
}

func (s *Service) RunBusinessProbe(ctx context.Context, identity requestcontext.Context, input generated.BusinessProbeInput) (generated.BusinessProbeOutput, error) {
	if err := runtime.RequirePermission(identity, runtime.PermissionIncidentRemediate); err != nil {
		return generated.BusinessProbeOutput{}, err
	}
	if s.remediation == nil {
		return generated.BusinessProbeOutput{}, runtime.NewError(runtime.AdapterNotConfigured, "Incident business probe is not configured", false)
	}
	value, err := s.remediation.RunBusinessProbe(ctx, identity, input.ProbeId)
	if err != nil {
		return generated.BusinessProbeOutput{}, err
	}
	return generated.BusinessProbeOutput{ProbeId: value.ProbeID, Result: generated.BusinessProbeOutputResult(value.Result), DurationMs: value.DurationMS, CompletedAt: value.CompletedAt}, nil
}

func operationOutput(value orderdemo.Operation) generated.OperationOutput {
	return generated.OperationOutput{OperationId: value.OperationID, InstanceEpoch: value.InstanceEpoch, BeforeVersion: value.BeforeVersion, AfterVersion: value.AfterVersion, BeforeConcurrency: value.BeforeConcurrency, AfterConcurrency: value.AfterConcurrency, IntentDigest: value.IntentDigest, ApprovalId: value.ApprovalID, ExecutedAt: value.ExecutedAt}
}

func authorize(identity requestcontext.Context) error {
	return runtime.RequirePermission(identity, runtime.PermissionIncidentDiagnose)
}

func authorizeReadOrRemediate(identity requestcontext.Context) error {
	if identity.HasPermission(runtime.PermissionIncidentDiagnose) || identity.HasPermission(runtime.PermissionIncidentRemediate) {
		return nil
	}
	return runtime.NewError(runtime.PermissionDenied, "required permission is missing", false)
}

func invalidInput() error {
	return runtime.NewError(runtime.SchemaValidationFailed, "incident tool input does not match the schema", false)
}

func mapValue(source, destination any) error {
	encoded, err := json.Marshal(source)
	if err != nil {
		return runtime.NewError(runtime.InternalError, "tool output could not be encoded", true)
	}
	if err := json.Unmarshal(encoded, destination); err != nil {
		return runtime.NewError(runtime.SchemaValidationFailed, "tool output does not match its schema", false)
	}
	return nil
}
