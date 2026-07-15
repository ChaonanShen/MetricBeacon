package executionaudit

import (
	"context"
	"time"
)

type Record struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenantId"`
	OrgID        string    `json:"orgId"`
	TaskID       string    `json:"taskId"`
	ApprovalID   string    `json:"approvalId"`
	IntentDigest string    `json:"intentDigest"`
	OperationID  string    `json:"operationId"`
	Phase        string    `json:"phase"`
	Outcome      string    `json:"outcome"`
	OccurredAt   time.Time `json:"occurredAt"`
}

type Port interface {
	Append(context.Context, Record) error
}
