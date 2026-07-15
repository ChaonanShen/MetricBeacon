package remediation

import (
	"strings"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type AuditAction string
type AuditOutcome string

const (
	AuditApprovalDecision   AuditAction = "approval_decision"
	AuditRemediationExecute AuditAction = "remediation_execute"
	AuditRemediationVerify  AuditAction = "remediation_verify"

	AuditAccepted  AuditOutcome = "accepted"
	AuditRejected  AuditOutcome = "rejected"
	AuditSucceeded AuditOutcome = "succeeded"
	AuditFailed    AuditOutcome = "failed"
)

type AuditRecord struct {
	ID         string
	TenantID   string
	OrgID      string
	TaskID     string
	ActorID    string
	Action     AuditAction
	Outcome    AuditOutcome
	Summary    string
	OccurredAt time.Time
}

func NewAuditRecord(id, tenantID, orgID, taskID, actorID string, action AuditAction, outcome AuditOutcome, summary string, now time.Time) (AuditRecord, error) {
	value := AuditRecord{ID: id, TenantID: tenantID, OrgID: orgID, TaskID: taskID, ActorID: strings.TrimSpace(actorID), Action: action, Outcome: outcome, Summary: strings.TrimSpace(summary), OccurredAt: now.UTC()}
	if !value.Valid() {
		return AuditRecord{}, common.NewError(common.InvalidArgument, "audit record is invalid", false)
	}
	return value, nil
}

func (a AuditRecord) Valid() bool {
	validAction := a.Action == AuditApprovalDecision || a.Action == AuditRemediationExecute || a.Action == AuditRemediationVerify
	validOutcome := a.Outcome == AuditAccepted || a.Outcome == AuditRejected || a.Outcome == AuditSucceeded || a.Outcome == AuditFailed
	return a.ID != "" && a.TenantID != "" && a.OrgID != "" && a.TaskID != "" && strings.TrimSpace(a.ActorID) != "" && validAction && validOutcome && strings.TrimSpace(a.Summary) != "" && len(a.Summary) <= 500 && !a.OccurredAt.IsZero()
}
