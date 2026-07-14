package task

import "testing"

func TestNewQueryPlanKeepsCanonicalViewOrder(t *testing.T) {
	plan, err := NewQueryPlan([]string{"memory", "cpu", "load"}, 5, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Views) != 3 || plan.Views[0] != "memory" || plan.Views[1] != "cpu" || plan.Views[2] != "load" {
		t.Fatalf("views = %#v", plan.Views)
	}
}

func TestNewQueryPlanRejectsDuplicateAndUnknownViews(t *testing.T) {
	for _, views := range [][]string{{"cpu", "cpu"}, {"disk"}} {
		if _, err := NewQueryPlan(views, 5, 30); err == nil {
			t.Fatalf("NewQueryPlan(%#v) succeeded", views)
		}
	}
}

func TestQueryPlanAllowsEmptyViewsForUnsupportedAndHistory(t *testing.T) {
	plan, err := NewQueryPlan(nil, 10, 60)
	if err != nil || !plan.Valid() || len(plan.Views) != 0 {
		t.Fatalf("plan = %#v, err = %v", plan, err)
	}
}
