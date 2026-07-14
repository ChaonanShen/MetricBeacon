package bootstrap_test

import (
	"fmt"
	"math"
	"testing"
	"time"
)

type liveMetricPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

type liveMetricSeries struct {
	Labels map[string]string `json:"labels"`
	Points []liveMetricPoint `json:"points"`
}

type liveMetricSummary struct {
	Series, Samples      int
	Min, Max             float64
	LatestMin, LatestMax float64
}

func summarizeLiveMetric(view string, series []liveMetricSeries) (liveMetricSummary, error) {
	if len(series) == 0 {
		return liveMetricSummary{}, fmt.Errorf("%s: no series", view)
	}
	if len(series) > 20 {
		return liveMetricSummary{}, fmt.Errorf("%s: exceeded 20 series", view)
	}
	summary := liveMetricSummary{Series: len(series), Min: math.Inf(1), Max: math.Inf(-1), LatestMin: math.Inf(1), LatestMax: math.Inf(-1)}
	type lastPoint struct {
		timestamp time.Time
		value     float64
	}
	lastPoints := make([]lastPoint, 0, len(series))
	for _, item := range series {
		if item.Labels["instance"] == "" {
			return liveMetricSummary{}, fmt.Errorf("%s: missing instance label", view)
		}
		if len(item.Points) == 0 {
			return liveMetricSummary{}, fmt.Errorf("%s: empty series", view)
		}
		previous := time.Time{}
		for index, point := range item.Points {
			timestamp, err := time.Parse(time.RFC3339Nano, point.Timestamp)
			if err != nil {
				return liveMetricSummary{}, fmt.Errorf("%s: invalid timestamp", view)
			}
			if !previous.IsZero() && !timestamp.After(previous) {
				return liveMetricSummary{}, fmt.Errorf("%s: timestamps were not strictly increasing", view)
			}
			previous = timestamp
			if math.IsNaN(point.Value) || math.IsInf(point.Value, 0) {
				return liveMetricSummary{}, fmt.Errorf("%s: non-finite value", view)
			}
			if (view == "cpu" || view == "memory") && (point.Value < 0 || point.Value > 100) {
				return liveMetricSummary{}, fmt.Errorf("%s: value outside 0..100", view)
			}
			if view == "load" && point.Value < 0 {
				return liveMetricSummary{}, fmt.Errorf("%s: value below zero", view)
			}
			summary.Samples++
			if summary.Samples > 5000 {
				return liveMetricSummary{}, fmt.Errorf("%s: exceeded 5000 samples", view)
			}
			summary.Min = math.Min(summary.Min, point.Value)
			summary.Max = math.Max(summary.Max, point.Value)
			if index == len(item.Points)-1 {
				lastPoints = append(lastPoints, lastPoint{timestamp: timestamp, value: point.Value})
			}
		}
	}
	latestTimestamp := lastPoints[0].timestamp
	for _, point := range lastPoints[1:] {
		if point.timestamp.After(latestTimestamp) {
			latestTimestamp = point.timestamp
		}
	}
	for _, point := range lastPoints {
		if point.timestamp.Equal(latestTimestamp) {
			summary.LatestMin = math.Min(summary.LatestMin, point.value)
			summary.LatestMax = math.Max(summary.LatestMax, point.value)
		}
	}
	return summary, nil
}

func TestSummarizeLiveMetric(t *testing.T) {
	valid := []liveMetricSeries{{Labels: map[string]string{"instance": "node:9100"}, Points: []liveMetricPoint{{Timestamp: "2026-07-14T00:00:00Z", Value: 20}, {Timestamp: "2026-07-14T00:05:00Z", Value: 25}}}}
	summary, err := summarizeLiveMetric("cpu", valid)
	if err != nil || summary.Series != 1 || summary.Samples != 2 || summary.Min != 20 || summary.Max != 25 || summary.LatestMin != 25 || summary.LatestMax != 25 {
		t.Fatalf("unexpected summary: %#v, %v", summary, err)
	}
	cases := []struct {
		name, view string
		series     []liveMetricSeries
	}{
		{name: "missing instance", view: "cpu", series: []liveMetricSeries{{Labels: map[string]string{}, Points: valid[0].Points}}},
		{name: "unordered", view: "cpu", series: []liveMetricSeries{{Labels: valid[0].Labels, Points: []liveMetricPoint{{Timestamp: "2026-07-14T00:05:00Z", Value: 1}, {Timestamp: "2026-07-14T00:00:00Z", Value: 2}}}}},
		{name: "CPU range", view: "cpu", series: []liveMetricSeries{{Labels: valid[0].Labels, Points: []liveMetricPoint{{Timestamp: "2026-07-14T00:00:00Z", Value: 101}}}}},
		{name: "negative load", view: "load", series: []liveMetricSeries{{Labels: valid[0].Labels, Points: []liveMetricPoint{{Timestamp: "2026-07-14T00:00:00Z", Value: -1}}}}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if _, err := summarizeLiveMetric(item.view, item.series); err == nil {
				t.Fatal("expected semantic validation error")
			}
		})
	}
}
