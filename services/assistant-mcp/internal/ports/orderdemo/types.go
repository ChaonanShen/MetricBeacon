package orderdemo

import "time"

type Runtime struct {
	ServiceRef       string
	InstanceEpoch    string
	StartedAt        time.Time
	SupervisorStatus string
}

type Queue struct {
	Depth            int
	Capacity         int
	OldestAgeSeconds float64
	ObservedAt       time.Time
}

type Worker struct {
	ServiceRef            string
	InstanceEpoch         string
	ConfiguredConcurrency int
	EffectiveConcurrency  int
	ActiveWorkers         int
	InflightOrders        int
	Version               int
	ObservedAt            time.Time
}

type Policy struct {
	ServiceRef          string
	ExpectedConcurrency int
	MinConcurrency      int
	MaxConcurrency      int
	Version             string
	Digest              string
}

type RecentRequest struct {
	Status string
	Limit  int
}

type OrderOutcome struct {
	ID            string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	FailureReason *string
}

type Operation struct {
	OperationID       string
	InstanceEpoch     string
	BeforeVersion     int
	AfterVersion      int
	BeforeConcurrency int
	AfterConcurrency  int
	IntentDigest      string
	ApprovalID        string
	ExecutedAt        time.Time
}

type RemediationRequest struct {
	OperationID         string
	InstanceEpoch       string
	ExpectedVersion     int
	ExpectedConcurrency int
	NewConcurrency      int
	IntentDigest        string
	ApprovalID          string
}

type ProbeResult struct {
	ProbeID     string
	Result      string
	DurationMS  int
	CompletedAt *time.Time
}
