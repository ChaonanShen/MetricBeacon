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
	validAction := action == AuditApprovalDecision || action == AuditRemediationExecute || action == AuditRemediationVerify
	validOutcome := outcome == AuditAccepted || outcome == AuditRejected || outcome == AuditSucceeded || outcome == AuditFailed
	if id == "" || tenantID == "" || orgID == "" || taskID == "" || strings.TrimSpace(actorID) == "" || !validAction || !validOutcome || strings.TrimSpace(summary) == "" || len(summary) > 500 || now.IsZero() {
		return AuditRecord{}, common.NewError(common.InvalidArgument, "audit record is invalid", false)
	}
	return AuditRecord{ID: id, TenantID: tenantID, OrgID: orgID, TaskID: taskID, ActorID: strings.TrimSpace(actorID), Action: action, Outcome: outcome, Summary: strings.TrimSpace(summary), OccurredAt: now.UTC()}, nil
}
