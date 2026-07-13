package mock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mini-torchbearing.local/services/assistant-mcp/internal/ports/prometheus"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

const (
	ScenarioID  = "node_exporter_overview"
	CPUQuery    = `100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`
	MemoryQuery = `100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes`
	LoadQuery   = `node_load1`
)

type fixture struct {
	Search  prometheus.SearchMetricsResult
	Labels  map[string]prometheus.MetricLabelsResult
	Queries map[string]prometheus.QueryResult
}

func loadFixture(directory string) (fixture, error) {
	manifest, err := os.ReadFile(filepath.Join(directory, "manifest.yaml"))
	if err != nil {
		return fixture{}, unavailable("mock scenario manifest is unavailable")
	}
	if !strings.Contains(string(manifest), "scenarioId: "+ScenarioID) {
		return fixture{}, runtime.NewError(runtime.SchemaValidationFailed, "mock scenario manifest is invalid", false)
	}

	var result fixture
	if err := decodeFixture(filepath.Join(directory, "search_metrics.json"), &result.Search); err != nil {
		return fixture{}, err
	}
	var labels struct {
		Labels []prometheus.MetricLabelsResult `json:"labels"`
	}
	if err := decodeFixture(filepath.Join(directory, "metric_labels.json"), &labels); err != nil {
		return fixture{}, err
	}
	result.Labels = make(map[string]prometheus.MetricLabelsResult, len(labels.Labels))
	for _, item := range labels.Labels {
		result.Labels[item.MetricName] = item
	}
	result.Queries = make(map[string]prometheus.QueryResult, 3)
	for _, entry := range []struct {
		file       string
		expression string
	}{{"query_cpu.json", CPUQuery}, {"query_memory.json", MemoryQuery}, {"query_load.json", LoadQuery}} {
		var query prometheus.QueryResult
		if err := decodeFixture(filepath.Join(directory, entry.file), &query); err != nil {
			return fixture{}, err
		}
		result.Queries[entry.expression] = query
	}
	if err := validateFixture(result); err != nil {
		return fixture{}, err
	}
	return result, nil
}

func decodeFixture(path string, destination any) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return unavailable("mock scenario fixture is unavailable")
	}
	if err := json.Unmarshal(contents, destination); err != nil {
		return runtime.NewError(runtime.SchemaValidationFailed, "mock scenario fixture does not match its schema", false)
	}
	return nil
}

func validateFixture(value fixture) error {
	if len(value.Search.Candidates) < 4 || len(value.Labels) != 3 || len(value.Queries) != 3 {
		return runtime.NewError(runtime.SchemaValidationFailed, "mock scenario fixture is incomplete", false)
	}
	for _, candidate := range value.Search.Candidates {
		if candidate.MetricName == "" || candidate.Type == "" || candidate.Description == "" || len(candidate.Labels) == 0 || len(candidate.Sources) != 1 || candidate.Sources[0].Type != "mock_fixture" || candidate.Sources[0].Reference != ScenarioID {
			return runtime.NewError(runtime.SchemaValidationFailed, "metric fixture is invalid", false)
		}
	}
	for _, name := range []string{"node_cpu_seconds_total", "node_memory_MemAvailable_bytes", "node_load1"} {
		labels, ok := value.Labels[name]
		if !ok || len(labels.LabelNames) == 0 || len(labels.SampleValues) == 0 {
			return runtime.NewError(runtime.SchemaValidationFailed, "metric labels fixture is invalid", false)
		}
	}
	for _, result := range value.Queries {
		if !result.Validation.Valid || result.Status != "success" || result.ResultType != "matrix" || len(result.Series) < 2 || result.DurationMS < 0 {
			return runtime.NewError(runtime.SchemaValidationFailed, "query fixture is invalid", false)
		}
		for _, series := range result.Series {
			if series.Name == "" || len(series.Points) < 6 {
				return runtime.NewError(runtime.SchemaValidationFailed, "query series fixture is invalid", false)
			}
			for _, point := range series.Points {
				if point.Timestamp.IsZero() {
					return runtime.NewError(runtime.SchemaValidationFailed, "query point fixture is invalid", false)
				}
			}
		}
	}
	return nil
}

func unavailable(message string) error {
	return runtime.NewError(runtime.DependencyUnavailable, message, true)
}

func shiftToRange(value prometheus.QueryResult, start time.Time) prometheus.QueryResult {
	if len(value.Series) == 0 || len(value.Series[0].Points) == 0 {
		return value
	}
	delta := start.UTC().Sub(value.Series[0].Points[0].Timestamp.UTC())
	result := value
	result.Series = make([]prometheus.Series, len(value.Series))
	for seriesIndex, series := range value.Series {
		copied := prometheus.Series{Name: series.Name, Labels: make(map[string]string, len(series.Labels)), Points: make([]prometheus.Point, len(series.Points))}
		for key, item := range series.Labels {
			copied.Labels[key] = item
		}
		for pointIndex, point := range series.Points {
			copied.Points[pointIndex] = prometheus.Point{Timestamp: point.Timestamp.Add(delta).UTC(), Value: point.Value}
		}
		result.Series[seriesIndex] = copied
	}
	return result
}
