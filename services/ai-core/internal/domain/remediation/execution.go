package remediation

import (
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type ExecutionState string

const (
	ExecutionStarted        ExecutionState = "started"
	ExecutionApplied        ExecutionState = "applied"
	ExecutionAlreadyApplied ExecutionState = "already_applied"
	ExecutionFailed         ExecutionState = "failed"
	ExecutionUnknown        ExecutionState = "unknown"
)

type Execution struct {
	OperationID     string
	TenantID        string
	OrgID           string
	TaskID          string
	ApprovalID      string
	IntentDigest    string
	InstanceEpoch   string
	ExpectedVersion int64
	State           ExecutionState
	BeforeVersion   *int64
	AfterVersion    *int64
	ErrorCode       *common.ErrorCode
	StartedAt       time.Time
	CompletedAt     *time.Time
	Version         int64
}

func NewExecution(operationID, tenantID, orgID, taskID, approvalID, intentDigest, instanceEpoch string, expectedVersion int64, now time.Time) (Execution, error) {
	if operationID == "" || tenantID == "" || orgID == "" || taskID == "" || approvalID == "" || !ValidDigest(intentDigest) || instanceEpoch == "" || expectedVersion < 1 || now.IsZero() {
		return Execution{}, common.NewError(common.InvalidArgument, "remediation execution is invalid", false)
	}
	return Execution{OperationID: operationID, TenantID: tenantID, OrgID: orgID, TaskID: taskID, ApprovalID: approvalID, IntentDigest: intentDigest, InstanceEpoch: instanceEpoch, ExpectedVersion: expectedVersion, State: ExecutionStarted, StartedAt: now.UTC(), Version: 1}, nil
}

func (e *Execution) RecordReceipt(beforeVersion, afterVersion int64, alreadyApplied bool, now time.Time) error {
	if (e.State != ExecutionStarted && e.State != ExecutionUnknown) || beforeVersion != e.ExpectedVersion || afterVersion != beforeVersion+1 || now.IsZero() || now.UTC().Before(e.StartedAt) {
		return common.NewError(common.InvalidStateTransition, "remediation receipt is invalid", false)
	}
	now = now.UTC()
	e.BeforeVersion, e.AfterVersion, e.CompletedAt = &beforeVersion, &afterVersion, &now
	if alreadyApplied {
		e.State = ExecutionAlreadyApplied
	} else {
		e.State = ExecutionApplied
	}
	e.ErrorCode = nil
	e.Version++
	return nil
}

func (e *Execution) MarkUnknown(now time.Time) error {
	return e.markIncomplete(ExecutionUnknown, nil, now)
}

func (e *Execution) MarkFailed(code common.ErrorCode, now time.Time) error {
	if code == "" {
		return common.NewError(common.InvalidArgument, "execution failure code is required", false)
	}
	return e.markIncomplete(ExecutionFailed, &code, now)
}

func (e *Execution) markIncomplete(state ExecutionState, code *common.ErrorCode, now time.Time) error {
	if e.State != ExecutionStarted || now.IsZero() || now.UTC().Before(e.StartedAt) {
		return common.NewError(common.InvalidStateTransition, "remediation execution transition is not allowed", false)
	}
	now = now.UTC()
	e.State, e.ErrorCode, e.CompletedAt = state, code, &now
	e.Version++
	return nil
}

func (e Execution) Valid() bool {
	if e.OperationID == "" || e.TenantID == "" || e.OrgID == "" || e.TaskID == "" || e.ApprovalID == "" || !ValidDigest(e.IntentDigest) || e.InstanceEpoch == "" || e.ExpectedVersion < 1 || e.StartedAt.IsZero() || e.Version < 1 {
		return false
	}
	switch e.State {
	case ExecutionStarted:
		return e.Version == 1 && e.BeforeVersion == nil && e.AfterVersion == nil && e.ErrorCode == nil && e.CompletedAt == nil
	case ExecutionApplied, ExecutionAlreadyApplied:
		return e.BeforeVersion != nil && e.AfterVersion != nil && *e.BeforeVersion == e.ExpectedVersion && *e.AfterVersion == *e.BeforeVersion+1 && e.ErrorCode == nil && e.CompletedAt != nil
	case ExecutionUnknown:
		return e.BeforeVersion == nil && e.AfterVersion == nil && e.ErrorCode == nil && e.CompletedAt != nil
	case ExecutionFailed:
		return e.BeforeVersion == nil && e.AfterVersion == nil && e.ErrorCode != nil && e.CompletedAt != nil
	default:
		return false
	}
}
