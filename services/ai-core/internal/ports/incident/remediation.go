package incident

import (
	"context"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
)

type RestoreRequest struct {
	OperationID, InstanceEpoch, IntentDigest, ApprovalID, ApprovalEvidence string
	ExpectedVersion                                                        int64
}

type OperationReceipt struct {
	OperationID, InstanceEpoch, IntentDigest, ApprovalID string
	BeforeVersion, AfterVersion                          int64
	BeforeConcurrency, AfterConcurrency                  int
	ExecutedAt                                           time.Time
}

type RuntimeState struct {
	ServiceRef, InstanceEpoch, SupervisorStatus string
	StartedAt                                   time.Time
}

type WorkerState struct {
	ServiceRef, InstanceEpoch                   string
	ConfiguredConcurrency, EffectiveConcurrency int
	ActiveWorkers, InflightOrders               int
	Version                                     int64
	ObservedAt                                  time.Time
}

type RecoveryMetrics struct {
	WindowSeconds                 int
	AcceptedDelta, CompletedDelta float64
	QueueDepth, OldestAgeSeconds  float64
	ObservedAt                    time.Time
}

type BusinessProbe struct {
	ProbeID, Result string
	DurationMS      int
	CompletedAt     time.Time
}

type RemediationToolset interface {
	RestoreWorkerConcurrency(context.Context, requestcontext.Context, RestoreRequest) (OperationReceipt, ToolEvidence, error)
	GetOperation(context.Context, requestcontext.Context, string) (OperationReceipt, ToolEvidence, error)
	GetRuntime(context.Context, requestcontext.Context) (RuntimeState, ToolEvidence, error)
	GetWorker(context.Context, requestcontext.Context) (WorkerState, ToolEvidence, error)
	GetRecoveryMetrics(context.Context, requestcontext.Context) (RecoveryMetrics, ToolEvidence, error)
	RunBusinessProbe(context.Context, requestcontext.Context, string) (BusinessProbe, ToolEvidence, error)
}
