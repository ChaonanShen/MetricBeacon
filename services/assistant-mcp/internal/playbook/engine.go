package playbook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/assets"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/orderdemo"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

const checkpointVersion = 1

type Engine struct {
	assets assets.Port
	orders orderdemo.Port
	key    []byte
	now    func() time.Time
}

type AssetRef struct {
	Kind    string
	ID      string
	Version string
	Digest  string
}

type StartResult struct {
	Status        string
	Checkpoint    string
	AssetRefs     []AssetRef
	CapabilityIDs []string
}

type Diagnosis struct {
	PrimaryHypothesis string
	EvidenceRefs      []string
	Alternatives      []string
	Confidence        float64
	CandidateAction   string
}

type IntentDraft struct {
	CapabilityID      string
	ServiceRef        string
	InstanceEpoch     string
	ExpectedVersion   int
	ObservedAt        time.Time
	PolicyDigest      string
	PlaybookDigest    string
	BeforeConcurrency int
	AfterConcurrency  int
	Risk              string
}

type ResumeResult struct {
	Status      string
	Checkpoint  string
	IntentDraft *IntentDraft
}

type checkpoint struct {
	Version         int              `json:"version"`
	Phase           string           `json:"phase"`
	PlaybookID      string           `json:"playbookId"`
	PlaybookVersion string           `json:"playbookVersion"`
	PlaybookDigest  string           `json:"playbookDigest"`
	KnowledgeDigest string           `json:"knowledgeDigest"`
	SkillDigest     string           `json:"skillDigest"`
	ServiceRef      string           `json:"serviceRef"`
	RuntimeEpoch    string           `json:"runtimeEpoch"`
	Worker          orderdemo.Worker `json:"worker"`
	Policy          orderdemo.Policy `json:"policy"`
	Queue           orderdemo.Queue  `json:"queue"`
	ObservedAt      time.Time        `json:"observedAt"`
}

func NewEngine(assetPort assets.Port, orderPort orderdemo.Port, checkpointKey []byte, now func() time.Time) (*Engine, error) {
	if assetPort == nil || orderPort == nil || len(checkpointKey) < 32 {
		return nil, fmt.Errorf("Playbook engine requires assets, orders, and a checkpoint key of at least 32 bytes")
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Engine{assets: assetPort, orders: orderPort, key: append([]byte{}, checkpointKey...), now: now}, nil
}

func (e *Engine) Start(ctx context.Context, identity requestcontext.Context, playbookID, version, digest, serviceRef string) (StartResult, error) {
	if playbookID == "" || version == "" || len(digest) != 64 || serviceRef != "order-demo" {
		return StartResult{}, runtime.NewError(runtime.SchemaValidationFailed, "Playbook start input is invalid", false)
	}
	definition, err := e.assets.Playbook(playbookID, version, digest)
	if err != nil {
		return StartResult{}, err
	}
	if definition.ServiceRef != serviceRef {
		return StartResult{}, runtime.NewError(runtime.SchemaValidationFailed, "Playbook target does not match the service", false)
	}
	knowledge, err := e.assets.Knowledge(definition.Knowledge.ID, definition.Knowledge.Version)
	if err != nil {
		return StartResult{}, err
	}
	skill, err := e.assets.Skill(definition.Skill.ID, definition.Skill.Version)
	if err != nil {
		return StartResult{}, err
	}
	runtimeState, err := e.orders.GetRuntime(ctx, identity)
	if err != nil {
		return StartResult{}, err
	}
	queue, err := e.orders.GetQueue(ctx, identity)
	if err != nil {
		return StartResult{}, err
	}
	worker, err := e.orders.GetWorker(ctx, identity)
	if err != nil {
		return StartResult{}, err
	}
	policy, err := e.orders.GetPolicy(ctx, identity)
	if err != nil {
		return StartResult{}, err
	}
	observedAt := e.now().UTC()
	if runtimeState.ServiceRef != serviceRef || worker.ServiceRef != serviceRef || policy.ServiceRef != serviceRef || runtimeState.InstanceEpoch == "" || runtimeState.InstanceEpoch != worker.InstanceEpoch || worker.ObservedAt.IsZero() || worker.ObservedAt.After(observedAt.Add(5*time.Second)) || queue.ObservedAt.IsZero() || queue.ObservedAt.After(observedAt.Add(5*time.Second)) {
		return StartResult{}, runtime.NewError(runtime.SchemaValidationFailed, "order observations are inconsistent", false)
	}
	state := checkpoint{Version: checkpointVersion, Phase: "needs_agent", PlaybookID: definition.ID, PlaybookVersion: definition.Version, PlaybookDigest: definition.Digest, KnowledgeDigest: knowledge.Digest, SkillDigest: skill.Digest, ServiceRef: serviceRef, RuntimeEpoch: runtimeState.InstanceEpoch, Worker: worker, Policy: policy, Queue: queue, ObservedAt: observedAt}
	encoded, err := e.sign(state)
	if err != nil {
		return StartResult{}, runtime.NewError(runtime.InternalError, "Playbook checkpoint could not be created", true)
	}
	return StartResult{
		Status: "needs_agent", Checkpoint: encoded,
		AssetRefs:     []AssetRef{{Kind: "knowledge", ID: knowledge.ID, Version: knowledge.Version, Digest: knowledge.Digest}, {Kind: "skill", ID: skill.ID, Version: skill.Version, Digest: skill.Digest}, {Kind: "playbook", ID: definition.ID, Version: definition.Version, Digest: definition.Digest}},
		CapabilityIDs: append([]string{}, definition.AllowedCapabilities...),
	}, nil
}

func (e *Engine) Resume(_ context.Context, _ requestcontext.Context, encoded string, diagnosis Diagnosis) (ResumeResult, error) {
	state, err := e.verify(encoded)
	if err != nil {
		return ResumeResult{}, err
	}
	if state.Phase != "needs_agent" {
		return ResumeResult{}, runtime.NewError(runtime.InvalidStateTransition, "Playbook checkpoint is not waiting for diagnosis", false)
	}
	definition, err := e.assets.Playbook(state.PlaybookID, state.PlaybookVersion, state.PlaybookDigest)
	if err != nil {
		return ResumeResult{}, err
	}
	knowledge, err := e.assets.Knowledge(definition.Knowledge.ID, definition.Knowledge.Version)
	if err != nil {
		return ResumeResult{}, err
	}
	skill, err := e.assets.Skill(definition.Skill.ID, definition.Skill.Version)
	if err != nil {
		return ResumeResult{}, err
	}
	if knowledge.Digest != state.KnowledgeDigest || skill.Digest != state.SkillDigest {
		return ResumeResult{}, runtime.NewError(runtime.ResourceConflict, "pinned operational assets changed", false)
	}
	if err := validateDiagnosis(diagnosis); err != nil {
		return ResumeResult{}, err
	}
	maximumAge := time.Duration(definition.PreparePolicy.MaxObservationAgeSeconds) * time.Second
	if age := e.now().UTC().Sub(state.Worker.ObservedAt); age < 0 || age > maximumAge {
		return ResumeResult{}, runtime.NewError(runtime.InvalidStateTransition, "Playbook observations are stale", false)
	}
	eligible := diagnosis.CandidateAction == definition.PreparePolicy.CandidateAction && diagnosis.Confidence >= 0.8 && hasEvidence(diagnosis.EvidenceRefs, "order_service.get_worker_state") && hasEvidence(diagnosis.EvidenceRefs, "order_service.get_worker_policy") && state.RuntimeEpoch == state.Worker.InstanceEpoch && state.Worker.ConfiguredConcurrency == definition.PreparePolicy.ConfiguredConcurrency && state.Worker.EffectiveConcurrency == definition.PreparePolicy.EffectiveConcurrency && state.Worker.ActiveWorkers == definition.PreparePolicy.ActiveWorkers && state.Policy.ExpectedConcurrency == definition.PreparePolicy.HealthyConcurrency
	if !eligible {
		state.Phase = "completed"
		checkpoint, signErr := e.sign(state)
		if signErr != nil {
			return ResumeResult{}, runtime.NewError(runtime.InternalError, "Playbook checkpoint could not be created", true)
		}
		return ResumeResult{Status: "completed", Checkpoint: checkpoint}, nil
	}
	state.Phase = "needs_approval"
	checkpoint, err := e.sign(state)
	if err != nil {
		return ResumeResult{}, runtime.NewError(runtime.InternalError, "Playbook checkpoint could not be created", true)
	}
	return ResumeResult{Status: "needs_approval", Checkpoint: checkpoint, IntentDraft: &IntentDraft{CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: state.ServiceRef, InstanceEpoch: state.RuntimeEpoch, ExpectedVersion: state.Worker.Version, ObservedAt: state.Worker.ObservedAt, PolicyDigest: state.Policy.Digest, PlaybookDigest: state.PlaybookDigest, BeforeConcurrency: 0, AfterConcurrency: 2, Risk: "Restarts bounded order processing at the pinned healthy concurrency; execution still requires fresh epoch/version checks and approval."}}, nil
}

func (e *Engine) sign(value checkpoint) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, e.key)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (e *Engine) verify(encoded string) (checkpoint, error) {
	parts := strings.Split(encoded, ".")
	if len(parts) != 2 || len(encoded) > 4096 {
		return checkpoint{}, invalidCheckpoint()
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return checkpoint{}, invalidCheckpoint()
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return checkpoint{}, invalidCheckpoint()
	}
	mac := hmac.New(sha256.New, e.key)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return checkpoint{}, invalidCheckpoint()
	}
	var value checkpoint
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil || value.Version != checkpointVersion || value.PlaybookID == "" || value.PlaybookDigest == "" || value.RuntimeEpoch == "" {
		return checkpoint{}, invalidCheckpoint()
	}
	return value, nil
}

func validateDiagnosis(value Diagnosis) error {
	if strings.TrimSpace(value.PrimaryHypothesis) == "" || len(value.PrimaryHypothesis) > 500 || len(value.EvidenceRefs) < 1 || len(value.EvidenceRefs) > 12 || len(value.Alternatives) > 5 || value.Confidence < 0 || value.Confidence > 1 || (value.CandidateAction != "restore_worker_concurrency" && value.CandidateAction != "no_action") {
		return runtime.NewError(runtime.SchemaValidationFailed, "diagnosis does not match the strict contract", false)
	}
	for _, reference := range value.EvidenceRefs {
		if _, allowed := readCapabilities[reference]; !allowed {
			return runtime.NewError(runtime.SchemaValidationFailed, "diagnosis contains an unknown evidence reference", false)
		}
	}
	return nil
}

var readCapabilities = map[string]struct{}{
	"order_service.get_runtime": {}, "order_service.get_queue_snapshot": {}, "order_service.get_worker_state": {}, "order_service.get_worker_policy": {}, "order_service.get_recent_outcomes": {}, "order_service.get_operation": {},
}

func hasEvidence(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func invalidCheckpoint() error {
	return runtime.NewError(runtime.SchemaValidationFailed, "Playbook checkpoint is invalid", false)
}
