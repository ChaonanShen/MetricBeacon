package chart

import (
	"testing"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

func TestChartTransitionsFromProposedToReady(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC)
	range30m, _ := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	chart, err := New("chart_1", "org:1", "session_1", "task_1", "CPU", "percent", []QuerySpec{{RefID: "A", Expression: "up", Legend: "{{instance}}", DatasourceUID: "prometheus-main", TimeRange: range30m, StepSeconds: 300}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := chart.MarkReady("execution_1", now); err != nil {
		t.Fatal(err)
	}
	if chart.Status != StatusReady || chart.LatestExecutionID == nil {
		t.Fatalf("unexpected chart: %#v", chart)
	}
	if err := chart.MarkReady("execution_2", now); err == nil {
		t.Fatal("ready chart transitioned again")
	}
}
