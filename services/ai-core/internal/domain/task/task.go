package task

import (
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type Status string

const (
	StatusCreated      Status = "created"
	StatusPlanning     Status = "planning"
	StatusRunningTools Status = "running_tools"
	StatusValidating   Status = "validating"
	StatusCompleted    Status = "completed"
	StatusFailed       Status = "failed"
)

type AnalysisTask struct {
	ID             string
	TenantID       string
	SessionID      string
	Status         Status
	InputMessageID string
	DatasourceUID  string
	TimeRange      common.AbsoluteTimeRange
	QueryPlan      QueryPlan
	LatestSequence int64
	Error          *common.DomainError
	CreatedAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	UpdatedAt      time.Time
	Version        int64
}

func New(id, tenantID, sessionID, inputMessageID, datasourceUID string, timeRange common.AbsoluteTimeRange, queryPlan QueryPlan, now time.Time) (AnalysisTask, error) {
	if id == "" || tenantID == "" || sessionID == "" || inputMessageID == "" || datasourceUID == "" {
		return AnalysisTask{}, common.NewError(common.InvalidArgument, "task identity, session, input message and datasource are required", false)
	}
	if !timeRange.From.Before(timeRange.To) {
		return AnalysisTask{}, common.NewError(common.InvalidArgument, "task time range is invalid", false)
	}
	if !queryPlan.Valid() {
		return AnalysisTask{}, common.NewError(common.InvalidArgument, "task query plan is invalid", false)
	}
	now = now.UTC()
	return AnalysisTask{ID: id, TenantID: tenantID, SessionID: sessionID, Status: StatusCreated, InputMessageID: inputMessageID, DatasourceUID: datasourceUID, TimeRange: timeRange, QueryPlan: queryPlan, CreatedAt: now, UpdatedAt: now, Version: 1}, nil
}

func (t *AnalysisTask) Transition(next Status, now time.Time) error {
	if !canTransition(t.Status, next) {
		return common.NewError(common.InvalidStateTransition, "task status transition is not allowed", false)
	}
	now = now.UTC()
	if t.StartedAt == nil && next != StatusCreated {
		t.StartedAt = &now
	}
	if next == StatusCompleted || next == StatusFailed {
		t.CompletedAt = &now
	}
	t.Status, t.UpdatedAt = next, now
	t.Version++
	return nil
}

func (t *AnalysisTask) Fail(err *common.DomainError, now time.Time) error {
	if err == nil || err.Code == "" {
		return common.NewError(common.InvalidArgument, "task failure requires an error code", false)
	}
	if transitionErr := t.Transition(StatusFailed, now); transitionErr != nil {
		return transitionErr
	}
	t.Error = err
	return nil
}

func canTransition(current, next Status) bool {
	switch current {
	case StatusCreated:
		return next == StatusPlanning || next == StatusFailed
	case StatusPlanning:
		return next == StatusRunningTools || next == StatusFailed
	case StatusRunningTools:
		return next == StatusValidating || next == StatusFailed
	case StatusValidating:
		return next == StatusCompleted || next == StatusFailed
	default:
		return false
	}
}
