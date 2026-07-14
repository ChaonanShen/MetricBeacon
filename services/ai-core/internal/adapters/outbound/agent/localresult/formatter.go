// Package localresult produces safe, deterministic summaries from query
// results. Model text never becomes the persisted factual answer.
package localresult

import (
	"fmt"
	"math"
	"strings"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/application/dto"
)

type Statistics struct {
	SeriesCount int
	SampleCount int
	First       float64
	Latest      float64
	Min         float64
	Max         float64
	Mean        float64
	Delta       float64
	ActualFrom  time.Time
	ActualTo    time.Time
	HasData     bool
}

func Summarize(proposal dto.ChartProposal) Statistics {
	result := Statistics{SeriesCount: len(proposal.Execution.Series)}
	var total float64
	var firstTotal, latestTotal float64
	var firstCount, latestCount int
	for _, series := range proposal.Execution.Series {
		for _, point := range series.Points {
			if math.IsNaN(point.Value) || math.IsInf(point.Value, 0) || point.Timestamp.IsZero() {
				continue
			}
			if !result.HasData || point.Timestamp.Before(result.ActualFrom) {
				result.ActualFrom = point.Timestamp.UTC()
				firstTotal, firstCount = point.Value, 1
			} else if point.Timestamp.Equal(result.ActualFrom) {
				firstTotal, firstCount = firstTotal+point.Value, firstCount+1
			}
			if !result.HasData || point.Timestamp.After(result.ActualTo) {
				result.ActualTo = point.Timestamp.UTC()
				latestTotal, latestCount = point.Value, 1
			} else if point.Timestamp.Equal(result.ActualTo) {
				latestTotal, latestCount = latestTotal+point.Value, latestCount+1
			}
			if !result.HasData {
				result.Min, result.Max = point.Value, point.Value
			} else {
				result.Min, result.Max = math.Min(result.Min, point.Value), math.Max(result.Max, point.Value)
			}
			result.HasData = true
			result.SampleCount++
			total += point.Value
		}
	}
	if result.HasData {
		result.First = firstTotal / float64(firstCount)
		result.Latest = latestTotal / float64(latestCount)
		result.Mean = total / float64(result.SampleCount)
		result.Delta = result.Latest - result.First
	}
	return result
}

func Format(request dto.AgentRunRequest, proposals []dto.ChartProposal) string {
	duration := request.TimeRange.To.Sub(request.TimeRange.From)
	header := fmt.Sprintf("已查询 node_exporter：范围 %s 至 %s（%s），step=%ds", timestamp(request.TimeRange.From), timestamp(request.TimeRange.To), durationText(duration), request.QueryPlan.StepSeconds)
	for _, proposal := range proposals {
		if proposal.Key == "cpu" {
			header += fmt.Sprintf("，CPU rate window=%ds", request.QueryPlan.CPURateWindowSeconds)
			break
		}
	}
	lines := []string{header + "。"}
	for _, proposal := range proposals {
		stats := Summarize(proposal)
		if !stats.HasData {
			lines = append(lines, fmt.Sprintf("- %s：查询成功，但没有可用样本。", proposal.Title))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s：%d 条序列、%d 个样本；首值 %s，最新 %s，变化 %s，最小 %s，最大 %s，均值 %s；实际数据 %s 至 %s。", proposal.Title, stats.SeriesCount, stats.SampleCount, value(proposal.Unit, stats.First), value(proposal.Unit, stats.Latest), signedValue(proposal.Unit, stats.Delta), value(proposal.Unit, stats.Min), value(proposal.Unit, stats.Max), value(proposal.Unit, stats.Mean), timestamp(stats.ActualFrom), timestamp(stats.ActualTo)))
	}
	return strings.Join(lines, "\n")
}

func timestamp(value time.Time) string { return value.UTC().Format("2006-01-02 15:04:05 UTC") }

func durationText(value time.Duration) string {
	if value%time.Hour == 0 {
		return fmt.Sprintf("%d小时", int(value/time.Hour))
	}
	if value%time.Minute == 0 {
		return fmt.Sprintf("%d分钟", int(value/time.Minute))
	}
	return fmt.Sprintf("%d秒", int(value/time.Second))
}

func value(unit string, number float64) string {
	if unit == "percent" {
		return fmt.Sprintf("%.2f%%", number)
	}
	return fmt.Sprintf("%.2f", number)
}

func signedValue(unit string, number float64) string {
	if unit == "percent" {
		return fmt.Sprintf("%+.2f 个百分点", number)
	}
	return fmt.Sprintf("%+.2f", number)
}
