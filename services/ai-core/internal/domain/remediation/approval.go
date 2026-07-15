package remediation

import (
	"strings"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

const DefaultApprovalTTL = 10 * time.Minute

type ApprovalStatus string
type Decision string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
	ApprovalExpired  ApprovalStatus = "expired"

	DecisionApprove Decision = "approve"
	DecisionReject  Decision = "reject"
)

type Approval struct {
	ID             string
	TenantID       string
	OrgID          string
	TaskID         string
	IntentID       string
	IntentDigest   string
	Status         ApprovalStatus
	RequestedAt    time.Time
	ExpiresAt      time.Time
	DecidedAt      *time.Time
	DecidedBy      *string
	DecisionReason *string
	Version        int64
}

func NewApproval(id, tenantID, orgID, taskID, intentID, intentDigest string, now time.Time) (Approval, error) {
	if id == "" || tenantID == "" || orgID == "" || taskID == "" || intentID == "" || !ValidDigest(intentDigest) || now.IsZero() {
		return Approval{}, common.NewError(common.InvalidArgument, "approval is invalid", false)
	}
	now = now.UTC()
	return Approval{ID: id, TenantID: tenantID, OrgID: orgID, TaskID: taskID, IntentID: intentID, IntentDigest: intentDigest, Status: ApprovalPending, RequestedAt: now, ExpiresAt: now.Add(DefaultApprovalTTL), Version: 1}, nil
}

func (a *Approval) Decide(decision Decision, actorID, reason string, now time.Time) error {
	if a.Status != ApprovalPending || (decision != DecisionApprove && decision != DecisionReject) || strings.TrimSpace(actorID) == "" || len(reason) > 500 || now.IsZero() || now.UTC().Before(a.RequestedAt) {
		return common.NewError(common.InvalidStateTransition, "approval decision is not allowed", false)
	}
	now = now.UTC()
	actorID = strings.TrimSpace(actorID)
	if !now.Before(a.ExpiresAt) {
		a.Status = ApprovalExpired
	} else if decision == DecisionApprove {
		a.Status = ApprovalApproved
	} else {
		a.Status = ApprovalRejected
	}
	a.DecidedAt, a.DecidedBy = &now, &actorID
	if reason = strings.TrimSpace(reason); reason != "" {
		a.DecisionReason = &reason
	}
	a.Version++
	return nil
}

func (a Approval) Valid() bool {
	if a.ID == "" || a.TenantID == "" || a.OrgID == "" || a.TaskID == "" || a.IntentID == "" || !ValidDigest(a.IntentDigest) || a.RequestedAt.IsZero() || !a.ExpiresAt.Equal(a.RequestedAt.Add(DefaultApprovalTTL)) || a.Version < 1 {
		return false
	}
	if a.Status == ApprovalPending {
		return a.Version == 1 && a.DecidedAt == nil && a.DecidedBy == nil && a.DecisionReason == nil
	}
	if (a.Status != ApprovalApproved && a.Status != ApprovalRejected && a.Status != ApprovalExpired) || a.Version != 2 || a.DecidedAt == nil || a.DecidedAt.Before(a.RequestedAt) || a.DecidedBy == nil || strings.TrimSpace(*a.DecidedBy) == "" || (a.DecisionReason != nil && len(*a.DecisionReason) > 500) {
		return false
	}
	if a.Status == ApprovalExpired {
		return !a.DecidedAt.Before(a.ExpiresAt)
	}
	return a.DecidedAt.Before(a.ExpiresAt)
}
