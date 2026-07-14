package task

import "mini-torchbearing.local/services/ai-core/internal/domain/common"

var allowedSteps = map[int]struct{}{5: {}, 10: {}, 15: {}, 30: {}, 60: {}, 120: {}, 300: {}}
var allowedCPUWindows = map[int]struct{}{30: {}, 60: {}, 300: {}}

type QueryPlan struct {
	StepSeconds          int
	CPURateWindowSeconds int
}

func NewQueryPlan(stepSeconds, cpuRateWindowSeconds int) (QueryPlan, error) {
	if _, ok := allowedSteps[stepSeconds]; !ok {
		return QueryPlan{}, common.NewError(common.InvalidArgument, "query step is outside the bounded policy", false)
	}
	if _, ok := allowedCPUWindows[cpuRateWindowSeconds]; !ok {
		return QueryPlan{}, common.NewError(common.InvalidArgument, "CPU rate window is outside the bounded policy", false)
	}
	return QueryPlan{StepSeconds: stepSeconds, CPURateWindowSeconds: cpuRateWindowSeconds}, nil
}

func (p QueryPlan) Valid() bool {
	_, stepOK := allowedSteps[p.StepSeconds]
	_, windowOK := allowedCPUWindows[p.CPURateWindowSeconds]
	return stepOK && windowOK
}

func LegacyQueryPlan() QueryPlan {
	return QueryPlan{StepSeconds: 300, CPURateWindowSeconds: 300}
}
