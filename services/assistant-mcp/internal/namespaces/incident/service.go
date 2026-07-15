package incident

import (
	"context"
	"encoding/json"

	generated "mini-torchbearing.local/packages/generated-contracts/go"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/playbook"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/assets"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/orderdemo"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

type Service struct {
	assets assets.Port
	orders orderdemo.Port
	engine *playbook.Engine
}

func NewService(assetPort assets.Port, orderPort orderdemo.Port, engine *playbook.Engine) *Service {
	return &Service{assets: assetPort, orders: orderPort, engine: engine}
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
	if err := authorize(identity); err != nil {
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
	if err := authorize(identity); err != nil {
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
	if err := authorize(identity); err != nil {
		return generated.OperationOutput{}, err
	}
	value, err := s.orders.GetOperation(ctx, identity, input.OperationId)
	if err != nil {
		return generated.OperationOutput{}, err
	}
	return generated.OperationOutput{OperationId: value.OperationID, InstanceEpoch: value.InstanceEpoch, BeforeVersion: value.BeforeVersion, AfterVersion: value.AfterVersion, BeforeConcurrency: value.BeforeConcurrency, AfterConcurrency: value.AfterConcurrency, IntentDigest: value.IntentDigest, ApprovalId: value.ApprovalID, ExecutedAt: value.ExecutedAt}, nil
}

func authorize(identity requestcontext.Context) error {
	return runtime.RequirePermission(identity, runtime.PermissionIncidentDiagnose)
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
