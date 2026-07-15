package task

import (
	"strings"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type Kind string
type Status string
type IncidentPhase string

const (
	KindMetricAnalysis      Kind = "metric_analysis"
	KindIncidentRemediation Kind = "incident_remediation"
)

const (
	StatusCreated         Status = "created"
	StatusPlanning        Status = "planning"
	StatusRunningTools    Status = "running_tools"
	StatusWaitingApproval Status = "waiting_approval"
	StatusExecuting       Status = "executing"
	StatusReconciling     Status = "reconciling"
	StatusValidating      Status = "validating"
	StatusCompleted       Status = "completed"
	StatusFailed          Status = "failed"
	StatusCancelled       Status = "cancelled"
)

const (
	PhaseLoadAssets     IncidentPhase = "load_assets"
	PhaseObserve        IncidentPhase = "observe"
	PhaseNeedsAgent     IncidentPhase = "needs_agent"
	PhasePrepare        IncidentPhase = "prepare"
	PhaseNeedsApproval  IncidentPhase = "needs_approval"
	PhaseExecute        IncidentPhase = "execute"
	PhaseVerifyRuntime  IncidentPhase = "verify_runtime"
	PhaseVerifyMetrics  IncidentPhase = "verify_metrics"
	PhaseVerifyBusiness IncidentPhase = "verify_business"
	PhaseCompleted      IncidentPhase = "completed"
	PhaseNoAction       IncidentPhase = "no_action"
)

type AssetRef struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

type PinnedRef struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
	Digest  string `json:"digest"`
}

type Diagnosis struct {
	PrimaryHypothesis     string   `json:"primaryHypothesis"`
	EvidenceRefs          []string `json:"evidenceRefs"`
	AlternativeHypotheses []string `json:"alternativeHypotheses"`
	Confidence            float64  `json:"confidence"`
	CandidateAction       string   `json:"candidateAction"`
}

type RemediationIntent struct {
	ID                string    `json:"id"`
	Digest            string    `json:"digest"`
	CapabilityID      string    `json:"capabilityId"`
	ServiceRef        string    `json:"serviceRef"`
	InstanceEpoch     string    `json:"instanceEpoch"`
	ExpectedVersion   int64     `json:"expectedVersion"`
	BeforeConcurrency int       `json:"beforeConcurrency"`
	AfterConcurrency  int       `json:"afterConcurrency"`
	Risk              string    `json:"risk"`
	CreatedAt         time.Time `json:"createdAt"`
}

type IncidentPlan struct {
	SourceID         string             `json:"sourceId"`
	AlertName        string             `json:"alertName"`
	AlertFingerprint string             `json:"alertFingerprint"`
	ServiceRef       string             `json:"serviceRef"`
	Labels           map[string]string  `json:"labels"`
	Mapping          PinnedRef          `json:"mapping"`
	Playbook         PinnedRef          `json:"playbook"`
	AssetRefs        []AssetRef         `json:"assetRefs"`
	Phase            IncidentPhase      `json:"phase"`
	Diagnosis        *Diagnosis         `json:"diagnosis"`
	Intent           *RemediationIntent `json:"intent"`
}

// AnalysisTask retains its historical name to avoid a broad package rename;
// Kind is the authoritative discriminator for the two mutually exclusive plans.
type AnalysisTask struct {
	ID             string
	TenantID       string
	Kind           Kind
	SessionID      string
	Status         Status
	InputMessageID string
	DatasourceUID  string
	TimeRange      common.AbsoluteTimeRange
	QueryPlan      QueryPlan
	IncidentPlan   *IncidentPlan
	LatestSequence int64
	Error          *common.DomainError
	CreatedAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	UpdatedAt      time.Time
	Version        int64
}

func New(id, tenantID, sessionID, inputMessageID, datasourceUID string, timeRange common.AbsoluteTimeRange, queryPlan QueryPlan, now time.Time) (AnalysisTask, error) {
	if id == "" || tenantID == "" || sessionID == "" || inputMessageID == "" || datasourceUID == "" {
		return AnalysisTask{}, common.NewError(common.InvalidArgument, "task identity, session, input message and datasource are required", false)
	}
	if !timeRange.From.Before(timeRange.To) {
		return AnalysisTask{}, common.NewError(common.InvalidArgument, "task time range is invalid", false)
	}
	if !queryPlan.Valid() {
		return AnalysisTask{}, common.NewError(common.InvalidArgument, "task query plan is invalid", false)
	}
	now = now.UTC()
	return AnalysisTask{ID: id, TenantID: tenantID, Kind: KindMetricAnalysis, SessionID: sessionID, Status: StatusCreated, InputMessageID: inputMessageID, DatasourceUID: datasourceUID, TimeRange: timeRange, QueryPlan: queryPlan, CreatedAt: now, UpdatedAt: now, Version: 1}, nil
}

func NewIncident(id, tenantID, sessionID, inputMessageID string, plan IncidentPlan, now time.Time) (AnalysisTask, error) {
	if id == "" || tenantID == "" || sessionID == "" || inputMessageID == "" {
		return AnalysisTask{}, common.NewError(common.InvalidArgument, "incident task identity, session and trigger message are required", false)
	}
	if err := validateIncidentPlan(plan); err != nil {
		return AnalysisTask{}, err
	}
	now = now.UTC()
	return AnalysisTask{ID: id, TenantID: tenantID, Kind: KindIncidentRemediation, SessionID: sessionID, Status: StatusCreated, InputMessageID: inputMessageID, IncidentPlan: &plan, CreatedAt: now, UpdatedAt: now, Version: 1}, nil
}

func (t *AnalysisTask) Transition(next Status, now time.Time) error {
	if !canTransition(t.Kind, t.Status, next) {
		return common.NewError(common.InvalidStateTransition, "task status transition is not allowed", false)
	}
	if t.Kind == KindIncidentRemediation && next == StatusWaitingApproval && (t.IncidentPlan == nil || t.IncidentPlan.Intent == nil) {
		return common.NewError(common.InvalidStateTransition, "incident approval requires an immutable intent", false)
	}
	now = now.UTC()
	if t.StartedAt == nil && next != StatusCreated {
		t.StartedAt = &now
	}
	if terminal(next) {
		t.CompletedAt = &now
	}
	t.Status, t.UpdatedAt = next, now
	if t.Kind == KindIncidentRemediation && t.IncidentPlan != nil {
		switch next {
		case StatusRunningTools:
			t.IncidentPlan.Phase = PhaseNeedsAgent
		case StatusWaitingApproval:
			t.IncidentPlan.Phase = PhaseNeedsApproval
		case StatusExecuting:
			t.IncidentPlan.Phase = PhaseExecute
		case StatusReconciling:
			t.IncidentPlan.Phase = PhaseVerifyRuntime
		case StatusValidating:
			t.IncidentPlan.Phase = PhaseVerifyMetrics
		case StatusCompleted:
			if t.IncidentPlan.Phase != PhaseNoAction {
				t.IncidentPlan.Phase = PhaseCompleted
			}
		}
	}
	t.Version++
	return nil
}

func (t *AnalysisTask) RecordDiagnosis(value Diagnosis, now time.Time) error {
	if t.Kind != KindIncidentRemediation || t.Status != StatusRunningTools || !validDiagnosis(value) {
		return common.NewError(common.InvalidStateTransition, "incident diagnosis cannot be recorded", false)
	}
	copyValue := value
	t.IncidentPlan.Diagnosis = &copyValue
	t.IncidentPlan.Phase = PhasePrepare
	t.UpdatedAt = now.UTC()
	t.Version++
	return nil
}

func (t *AnalysisTask) RecordIntent(value RemediationIntent, now time.Time) error {
	if t.Kind != KindIncidentRemediation || t.Status != StatusRunningTools || t.IncidentPlan == nil || t.IncidentPlan.Diagnosis == nil || !validIntent(value, t.IncidentPlan.ServiceRef) || t.IncidentPlan.Diagnosis.CandidateAction != "restore_worker_concurrency" {
		return common.NewError(common.InvalidStateTransition, "incident remediation intent cannot be recorded", false)
	}
	copyValue := value
	copyValue.CreatedAt = copyValue.CreatedAt.UTC()
	t.IncidentPlan.Intent = &copyValue
	t.IncidentPlan.Phase = PhaseNeedsApproval
	t.UpdatedAt = now.UTC()
	t.Version++
	return nil
}

func (t *AnalysisTask) CompleteNoAction(now time.Time) error {
	if t.Kind != KindIncidentRemediation || t.Status != StatusRunningTools || t.IncidentPlan == nil || t.IncidentPlan.Diagnosis == nil || t.IncidentPlan.Diagnosis.CandidateAction != "no_action" || t.IncidentPlan.Intent != nil {
		return common.NewError(common.InvalidStateTransition, "incident is not eligible for no-action completion", false)
	}
	t.IncidentPlan.Phase = PhaseNoAction
	return t.Transition(StatusCompleted, now)
}

func (t *AnalysisTask) Fail(err *common.DomainError, now time.Time) error {
	if err == nil || err.Code == "" {
		return common.NewError(common.InvalidArgument, "task failure requires an error code", false)
	}
	if transitionErr := t.Transition(StatusFailed, now); transitionErr != nil {
		return transitionErr
	}
	t.Error = err
	return nil
}

func canTransition(kind Kind, current, next Status) bool {
	if next == StatusFailed || next == StatusCancelled {
		return !terminal(current)
	}
	if kind == KindMetricAnalysis {
		switch current {
		case StatusCreated:
			return next == StatusPlanning
		case StatusPlanning:
			return next == StatusRunningTools
		case StatusRunningTools:
			return next == StatusValidating
		case StatusValidating:
			return next == StatusCompleted
		default:
			return false
		}
	}
	switch current {
	case StatusCreated:
		return next == StatusPlanning
	case StatusPlanning:
		return next == StatusRunningTools
	case StatusRunningTools:
		return next == StatusWaitingApproval || next == StatusCompleted
	case StatusWaitingApproval:
		return next == StatusExecuting
	case StatusExecuting:
		return next == StatusReconciling
	case StatusReconciling:
		return next == StatusValidating
	case StatusValidating:
		return next == StatusCompleted
	default:
		return false
	}
}

func terminal(status Status) bool {
	return status == StatusCompleted || status == StatusFailed || status == StatusCancelled
}

func validateIncidentPlan(plan IncidentPlan) error {
	if strings.TrimSpace(plan.SourceID) == "" || strings.TrimSpace(plan.AlertName) == "" || strings.TrimSpace(plan.AlertFingerprint) == "" || strings.TrimSpace(plan.ServiceRef) == "" || len(plan.Labels) == 0 || len(plan.Labels) > 24 || plan.Mapping.ID == "" || !validDigest(plan.Mapping.Digest) || plan.Playbook.ID == "" || plan.Playbook.Version == "" || !validDigest(plan.Playbook.Digest) || len(plan.AssetRefs) < 3 || len(plan.AssetRefs) > 8 || plan.Phase != PhaseNeedsAgent || plan.Diagnosis != nil || plan.Intent != nil {
		return common.NewError(common.InvalidArgument, "incident plan is invalid", false)
	}
	for key, value := range plan.Labels {
		if strings.TrimSpace(key) == "" || len(value) > 200 {
			return common.NewError(common.InvalidArgument, "incident labels are invalid", false)
		}
	}
	for _, ref := range plan.AssetRefs {
		if ref.ID == "" || ref.Version == "" || !validDigest(ref.Digest) {
			return common.NewError(common.InvalidArgument, "incident asset reference is invalid", false)
		}
	}
	return nil
}

func validDiagnosis(value Diagnosis) bool {
	validHypothesis := value.PrimaryHypothesis == "worker_stopped" || value.PrimaryHypothesis == "slow_processing" || value.PrimaryHypothesis == "dependency_errors" || value.PrimaryHypothesis == "healthy" || value.PrimaryHypothesis == "insufficient_evidence"
	validAction := value.CandidateAction == "restore_worker_concurrency" || value.CandidateAction == "no_action"
	return validHypothesis && validAction && len(value.EvidenceRefs) > 0 && len(value.EvidenceRefs) <= 12 && value.Confidence >= 0 && value.Confidence <= 1 && (value.PrimaryHypothesis == "worker_stopped" || value.CandidateAction == "no_action")
}

func validIntent(value RemediationIntent, serviceRef string) bool {
	return value.ID != "" && validDigest(value.Digest) && value.CapabilityID == "order_service.restore_worker_concurrency" && value.ServiceRef == serviceRef && value.InstanceEpoch != "" && value.ExpectedVersion >= 1 && value.BeforeConcurrency == 0 && value.AfterConcurrency == 2 && value.Risk == "low" && !value.CreatedAt.IsZero()
}

func validDigest(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[len("sha256:"):] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
