package chart

import (
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type Point struct {
	Timestamp time.Time
	Value     float64
}
type Series struct {
	Name   string
	Labels map[string]string
	Points []Point
}
type ExecutionStatus string

const (
	ExecutionSuccess ExecutionStatus = "success"
	ExecutionFailed  ExecutionStatus = "failed"
)

type Execution struct {
	ID          string
	TenantID    string
	ChartID     string
	QueryRefID  string
	Status      ExecutionStatus
	Series      []Series
	DurationMS  int64
	SampleRange common.AbsoluteTimeRange
	Warnings    []string
	CreatedAt   time.Time
}
