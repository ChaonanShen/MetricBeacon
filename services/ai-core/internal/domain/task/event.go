package task

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventTaskCreated             EventType = "task.created"
	EventTaskStatusChanged       EventType = "task.status_changed"
	EventAssistantMessageStarted EventType = "assistant.message.started"
	EventAssistantMessageDelta   EventType = "assistant.message.delta"
	EventAssistantMessageDone    EventType = "assistant.message.completed"
	EventToolStarted             EventType = "tool.started"
	EventToolCompleted           EventType = "tool.completed"
	EventToolFailed              EventType = "tool.failed"
	EventMetricCandidatesCreated EventType = "metric.candidates_created"
	EventChartCreated            EventType = "chart.created"
	EventChartExecutionDone      EventType = "chart.execution_completed"
	EventTaskCompleted           EventType = "task.completed"
	EventTaskFailed              EventType = "task.failed"
	EventAlertReceived           EventType = "alert.received"
	EventPlaybookResolved        EventType = "playbook.resolved"
	EventAssetsPinned            EventType = "assets.pinned"
	EventDiagnosisCompleted      EventType = "diagnosis.completed"
	EventIntentPrepared          EventType = "intent.prepared"
	EventApprovalRequested       EventType = "approval.requested"
	EventApprovalDecided         EventType = "approval.decided"
	EventRemediationStarted      EventType = "remediation.started"
	EventRemediationReconciled   EventType = "remediation.reconciled"
	EventVerificationRuntime     EventType = "verification.runtime"
	EventVerificationMetrics     EventType = "verification.metrics"
	EventVerificationBusiness    EventType = "verification.business"
	EventAuditRecorded           EventType = "audit.recorded"
)

type EventDraft struct {
	EventID   string
	TenantID  string
	TaskID    string
	SessionID string
	Type      EventType
	Timestamp time.Time
	Payload   json.RawMessage
}

type TaskEvent struct {
	EventDraft
	Sequence int64
}
