package commands

import (
	"testing"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

func TestResolveQueryPlanFromBoundedMessage(t *testing.T) {
	now := time.Date(2026, 7, 14, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name, message              string
		rangeSeconds, step, window int
	}{
		{"seconds", "查看近30s里 node exporter 中 cpu 数据", 30, 5, 30},
		{"minute", "查看近一分钟 CPU 变化图", 60, 5, 30},
		{"thirty minutes", "查看近30分钟 CPU 变化数据", 1800, 10, 60},
		{"explicit resolution", "最近五分钟 CPU，每个5s画一个点", 300, 5, 30},
		{"three views do not parse as time", "画出三种 node exporter 监测数据", 1800, 10, 60},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			timeRange, plan, err := ResolveQueryPlan(test.message, RequestedTimeRange{}, nil, now)
			if err != nil {
				t.Fatal(err)
			}
			if got := int(timeRange.To.Sub(timeRange.From) / time.Second); got != test.rangeSeconds || plan.StepSeconds != test.step || plan.CPURateWindowSeconds != test.window {
				t.Fatalf("resolved range=%d step=%d window=%d", got, plan.StepSeconds, plan.CPURateWindowSeconds)
			}
		})
	}
}

func TestResolveQueryPlanPrecedenceAndLimits(t *testing.T) {
	now := time.Date(2026, 7, 14, 15, 0, 0, 0, time.UTC)
	absolute, _ := common.NewAbsoluteTimeRange(now.Add(-2*time.Hour), now.Add(-time.Hour))
	step := 60
	timeRange, plan, err := ResolveQueryPlan("查看近30s CPU，每5s一个点", RequestedTimeRange{Absolute: &absolute}, &step, now)
	if err != nil {
		t.Fatal(err)
	}
	if timeRange != absolute || plan.StepSeconds != 5 || plan.CPURateWindowSeconds != 60 {
		t.Fatalf("absolute range or message step precedence failed: %#v %#v", timeRange, plan)
	}

	tooDense := 5
	if _, _, err := ResolveQueryPlan("查看近6小时 CPU", RequestedTimeRange{}, &tooDense, now); err == nil {
		t.Fatal("expected explicit resolution budget failure")
	}
	if _, _, err := ResolveQueryPlan("查看近10秒 CPU", RequestedTimeRange{}, nil, now); err == nil {
		t.Fatal("expected minimum range failure")
	}
}

func TestResolvePlannedQueryPlanPrecedenceAndViews(t *testing.T) {
	now := time.Date(2026, 7, 14, 15, 0, 0, 0, time.UTC)
	plannedRange := time.Minute
	plannedStep, requestedStep := 5, 30
	timeRange, plan, err := ResolvePlannedQueryPlan([]string{"cpu"}, &plannedRange, &plannedStep, RequestedTimeRange{RelativeDuration: 30 * time.Minute}, &requestedStep, now)
	if err != nil {
		t.Fatal(err)
	}
	if timeRange.To.Sub(timeRange.From) != time.Minute || plan.StepSeconds != 5 || plan.CPURateWindowSeconds != 30 || len(plan.Views) != 1 || plan.Views[0] != "cpu" {
		t.Fatalf("planned intent did not override API hints: range=%s plan=%#v", timeRange.To.Sub(timeRange.From), plan)
	}
	absolute, _ := common.NewAbsoluteTimeRange(now.Add(-2*time.Hour), now.Add(-time.Hour))
	_, plan, err = ResolvePlannedQueryPlan([]string{"memory"}, &plannedRange, nil, RequestedTimeRange{Absolute: &absolute}, &requestedStep, now)
	if err != nil || plan.CPURateWindowSeconds != 60 || plan.StepSeconds != 30 {
		t.Fatalf("absolute/API fallback precedence failed: %#v, %v", plan, err)
	}
}
