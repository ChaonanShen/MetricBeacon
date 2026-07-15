package mock

import (
	"context"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
)

func TestPlannerParsesExactChineseCadenceAndView(t *testing.T) {
	plan, err := (Planner{}).Plan(context.Background(), requestcontext.Context{}, agent.IntentPlanRequest{Message: "查看最近1分钟cpu的使用率变化，每隔5s采集个数据"})
	if err != nil || plan.Status != agent.IntentPlanned || len(plan.Views) != 1 || plan.Views[0] != "cpu" || plan.RangeDuration == nil || *plan.RangeDuration != time.Minute || plan.StepSeconds == nil || *plan.StepSeconds != 5 {
		t.Fatalf("plan = %#v, err = %v", plan, err)
	}
}

func TestPlannerUsesHistoryOnlyForOmittedView(t *testing.T) {
	plan, err := (Planner{}).Plan(context.Background(), requestcontext.Context{}, agent.IntentPlanRequest{Message: "那改成每隔30s", PreviousIntents: []agent.IntentHistoryItem{{Message: "查看 CPU", Views: []string{"cpu"}, RangeSeconds: 1800, StepSeconds: 10}}})
	if err != nil || len(plan.Views) != 1 || plan.Views[0] != "cpu" || plan.StepSeconds == nil || *plan.StepSeconds != 30 {
		t.Fatalf("plan = %#v, err = %v", plan, err)
	}
}

func TestPlannerReturnsUnsupportedWithoutARegisteredView(t *testing.T) {
	plan, err := (Planner{}).Plan(context.Background(), requestcontext.Context{}, agent.IntentPlanRequest{Message: "查看磁盘"})
	if err != nil || plan.Status != agent.IntentUnsupported || len(plan.Views) != 0 {
		t.Fatalf("plan = %#v, err = %v", plan, err)
	}
}
