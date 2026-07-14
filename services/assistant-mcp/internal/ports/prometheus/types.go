package prometheus

import "time"

type SearchMetricsRequest struct {
	DatasourceUID string
	Query         string
	Limit         int
}

type MetricSource struct {
	Type      string `json:"type"`
	Reference string `json:"reference"`
}

type MetricCandidate struct {
	MetricName  string
	Type        string
	Description string
	Labels      []string
	Score       float64
	Sources     []MetricSource
}

type SearchMetricsResult struct{ Candidates []MetricCandidate }

type GetMetricLabelsRequest struct {
	DatasourceUID string
	MetricName    string
}

type MetricLabelsResult struct {
	MetricName   string
	LabelNames   []string
	SampleValues map[string][]string
}

type QueryMode string

const (
	ModeValidate QueryMode = "validate"
	ModeExecute  QueryMode = "execute"
)

type QueryRequest struct {
	DatasourceUID string
	View          string
	// CPURateWindowSeconds must be set to 30, 60 or 300 for the CPU
	// view and must be nil for memory/load.
	CPURateWindowSeconds *int
	Start                time.Time
	End                  time.Time
	StepSeconds          int
	Mode                 QueryMode
}

type Validation struct {
	Valid               bool
	Errors              []string
	Warnings            []string
	MetricNames         []string
	LabelNames          []string
	CanonicalExpression string
}

type Point struct {
	Timestamp time.Time
	Value     float64
}

type Series struct {
	Name   string
	Labels map[string]string
	Points []Point
}

type QueryResult struct {
	Validation Validation
	Status     string
	ResultType string
	Series     []Series
	DurationMS int
	Warnings   []string
}
