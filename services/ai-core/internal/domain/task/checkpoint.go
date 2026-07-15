package task

import (
	"strings"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

const maxCheckpointBytes = 16 * 1024

// Checkpoint is an opaque, resumable playbook cursor. AI Core persists it but
// never exposes it through HTTP, TaskEvents, model context or ApprovalEvidence.
type Checkpoint struct {
	TaskID      string
	TenantID    string
	Phase       IncidentPhase
	OpaqueValue string
	UpdatedAt   time.Time
	Version     int64
}

func NewCheckpoint(taskID, tenantID string, phase IncidentPhase, opaqueValue string, now time.Time) (Checkpoint, error) {
	if taskID == "" || tenantID == "" || !validIncidentPhase(phase) || strings.TrimSpace(opaqueValue) == "" || len(opaqueValue) > maxCheckpointBytes {
		return Checkpoint{}, common.NewError(common.InvalidArgument, "task checkpoint is invalid", false)
	}
	return Checkpoint{TaskID: taskID, TenantID: tenantID, Phase: phase, OpaqueValue: opaqueValue, UpdatedAt: now.UTC(), Version: 1}, nil
}

func (c *Checkpoint) Replace(phase IncidentPhase, opaqueValue string, now time.Time) error {
	if !validIncidentPhase(phase) || strings.TrimSpace(opaqueValue) == "" || len(opaqueValue) > maxCheckpointBytes || now.UTC().Before(c.UpdatedAt) {
		return common.NewError(common.InvalidArgument, "task checkpoint update is invalid", false)
	}
	c.Phase, c.OpaqueValue, c.UpdatedAt = phase, opaqueValue, now.UTC()
	c.Version++
	return nil
}

func validIncidentPhase(phase IncidentPhase) bool {
	switch phase {
	case PhaseLoadAssets, PhaseObserve, PhaseNeedsAgent, PhasePrepare, PhaseNeedsApproval, PhaseExecute, PhaseVerifyRuntime, PhaseVerifyMetrics, PhaseVerifyBusiness, PhaseCompleted, PhaseNoAction:
		return true
	default:
		return false
	}
}
