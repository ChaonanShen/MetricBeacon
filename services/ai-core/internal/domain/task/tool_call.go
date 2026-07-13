package task

import (
	"encoding/json"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type ToolCallStatus string

const (
	ToolCallStarted   ToolCallStatus = "started"
	ToolCallCompleted ToolCallStatus = "completed"
	ToolCallFailed    ToolCallStatus = "failed"
)

type ToolCallRecord struct {
	ID            string
	TenantID      string
	TaskID        string
	ToolName      string
	ToolVersion   string
	Status        ToolCallStatus
	InputSummary  json.RawMessage
	OutputSummary json.RawMessage
	Error         *common.DomainError
	StartedAt     time.Time
	CompletedAt   *time.Time
	DurationMS    *int64
	Version       int64
}
