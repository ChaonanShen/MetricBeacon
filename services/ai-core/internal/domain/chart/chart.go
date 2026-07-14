package chart

import (
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type Status string

const (
	StatusProposed Status = "proposed"
	StatusReady    Status = "ready"
)

type QuerySpec struct {
	RefID         string
	Expression    string
	Legend        string
	DatasourceUID string
	TimeRange     common.AbsoluteTimeRange
	StepSeconds   int
}

type ChartDraft struct {
	ID                string
	TenantID          string
	SessionID         string
	TaskID            string
	Title             string
	Visualization     string
	Unit              string
	Queries           []QuerySpec
	Status            Status
	LatestExecutionID *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Version           int64
}

func New(id, tenantID, sessionID, taskID, title, unit string, queries []QuerySpec, now time.Time) (ChartDraft, error) {
	if id == "" || tenantID == "" || sessionID == "" || taskID == "" || title == "" || unit == "" || len(queries) == 0 {
		return ChartDraft{}, common.NewError(common.InvalidArgument, "chart identity, title, unit and query are required", false)
	}
	for _, query := range queries {
		if query.RefID == "" || query.Expression == "" || query.DatasourceUID == "" || !query.TimeRange.From.Before(query.TimeRange.To) || !validStep(query.StepSeconds) {
			return ChartDraft{}, common.NewError(common.InvalidArgument, "chart query is invalid", false)
		}
	}
	now = now.UTC()
	return ChartDraft{ID: id, TenantID: tenantID, SessionID: sessionID, TaskID: taskID, Title: title, Visualization: "timeseries", Unit: unit, Queries: queries, Status: StatusProposed, CreatedAt: now, UpdatedAt: now, Version: 1}, nil
}

func validStep(value int) bool {
	switch value {
	case 5, 10, 15, 30, 60, 120, 300:
		return true
	default:
		return false
	}
}

func (c *ChartDraft) MarkReady(executionID string, now time.Time) error {
	if c.Status != StatusProposed || executionID == "" {
		return common.NewError(common.InvalidStateTransition, "chart cannot transition to ready", false)
	}
	now = now.UTC()
	c.Status, c.LatestExecutionID, c.UpdatedAt, c.Version = StatusReady, &executionID, now, c.Version+1
	return nil
}
