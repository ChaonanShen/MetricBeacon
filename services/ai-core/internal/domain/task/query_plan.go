package task

import "mini-torchbearing.local/services/ai-core/internal/domain/common"

var allowedSteps = map[int]struct{}{5: {}, 10: {}, 15: {}, 30: {}, 60: {}, 120: {}, 300: {}}
var allowedCPUWindows = map[int]struct{}{30: {}, 60: {}, 300: {}}

type QueryPlan struct {
	Views                []string
	StepSeconds          int
	CPURateWindowSeconds int
}

func NewQueryPlan(views []string, stepSeconds, cpuRateWindowSeconds int) (QueryPlan, error) {
	normalized := make([]string, 0, len(views))
	seen := make(map[string]struct{}, len(views))
	for _, view := range views {
		if view != "cpu" && view != "memory" && view != "load" {
			return QueryPlan{}, common.NewError(common.InvalidArgument, "query view is outside the bounded registry", false)
		}
		if _, exists := seen[view]; exists {
			return QueryPlan{}, common.NewError(common.InvalidArgument, "query views must be unique", false)
		}
		seen[view] = struct{}{}
		normalized = append(normalized, view)
	}
	if _, ok := allowedSteps[stepSeconds]; !ok {
		return QueryPlan{}, common.NewError(common.InvalidArgument, "query step is outside the bounded policy", false)
	}
	if _, ok := allowedCPUWindows[cpuRateWindowSeconds]; !ok {
		return QueryPlan{}, common.NewError(common.InvalidArgument, "CPU rate window is outside the bounded policy", false)
	}
	return QueryPlan{Views: normalized, StepSeconds: stepSeconds, CPURateWindowSeconds: cpuRateWindowSeconds}, nil
}

func (p QueryPlan) Valid() bool {
	seen := make(map[string]struct{}, len(p.Views))
	for _, view := range p.Views {
		if view != "cpu" && view != "memory" && view != "load" {
			return false
		}
		if _, exists := seen[view]; exists {
			return false
		}
		seen[view] = struct{}{}
	}
	_, stepOK := allowedSteps[p.StepSeconds]
	_, windowOK := allowedCPUWindows[p.CPURateWindowSeconds]
	return stepOK && windowOK
}

func LegacyQueryPlan() QueryPlan {
	return QueryPlan{Views: []string{}, StepSeconds: 300, CPURateWindowSeconds: 300}
}
