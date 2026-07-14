package localresult

import (
	"strings"
	"testing"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
)

func TestFormatUsesEffectivePlanAndActualStatistics(t *testing.T) {
	from := time.Date(2026, 7, 14, 6, 0, 0, 0, time.UTC)
	request := dto.AgentRunRequest{TimeRange: common.AbsoluteTimeRange{From: from, To: from.Add(time.Minute)}, QueryPlan: task.QueryPlan{StepSeconds: 5, CPURateWindowSeconds: 30}}
	proposal := dto.ChartProposal{Key: "cpu", Title: "CPU 使用率", Unit: "percent", Execution: dto.QueryExecutionResult{Series: []chart.Series{
		{Points: []chart.Point{{Timestamp: from, Value: 10}, {Timestamp: from.Add(5 * time.Second), Value: 20}}},
		{Points: []chart.Point{{Timestamp: from, Value: 30}, {Timestamp: from.Add(5 * time.Second), Value: 50}}},
	}}}
	answer := Format(request, []dto.ChartProposal{proposal})
	for _, expected := range []string{"1分钟", "step=5s", "CPU rate window=30s", "2 条序列、4 个样本", "首值 20.00%", "最新 35.00%", "变化 +15.00 个百分点", "均值 27.50%"} {
		if !strings.Contains(answer, expected) {
			t.Fatalf("answer did not contain %q: %s", expected, answer)
		}
	}
	if strings.Contains(answer, "instance") {
		t.Fatalf("answer leaked labels: %s", answer)
	}
}
