// Package registry owns the narrowly-scoped node_exporter query allowlist.
// It belongs to the Prometheus adapter boundary so parser types never reach
// MCP handlers, ports, or AI Core.
package registry

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/prometheus/prometheus/promql/parser"
)

const (
	DatasourceUID       = "prometheus-main"
	MaxExpressionLength = 2048
	MaxASTNodes         = 32
	MaxSelectors        = 2
)

type Definition struct {
	View                string
	Title               string
	Unit                string
	RefID               string
	CanonicalExpression string
	MetricNames         []string
	LabelNames          []string
}

var definitions = []Definition{
	{View: "cpu", Title: "CPU 使用率", Unit: "percent", RefID: "A", CanonicalExpression: cpuExpression(300), MetricNames: []string{"node_cpu_seconds_total"}, LabelNames: []string{"instance", "mode"}},
	{View: "memory", Title: "内存可用率", Unit: "percent", RefID: "B", CanonicalExpression: `100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes`, MetricNames: []string{"node_memory_MemAvailable_bytes", "node_memory_MemTotal_bytes"}, LabelNames: []string{"instance"}},
	{View: "load", Title: "系统负载", Unit: "short", RefID: "C", CanonicalExpression: `node_load1`, MetricNames: []string{"node_load1"}, LabelNames: []string{"instance"}},
}

var cpuWindows = []int{30, 60, 300}

var registeredMetrics = map[string]struct{}{
	"node_cpu_seconds_total":         {},
	"node_memory_MemAvailable_bytes": {},
	"node_memory_MemTotal_bytes":     {},
	"node_load1":                     {},
}

func Definitions() []Definition {
	result := make([]Definition, len(definitions))
	copy(result, definitions)
	return result
}

func IsRegisteredMetric(metric string) bool {
	_, ok := registeredMetrics[metric]
	return ok
}

// Resolve renders the canonical PromQL for a registered view. Callers choose
// only bounded semantic parameters; raw PromQL never crosses the Port.
func Resolve(view string, cpuRateWindowSeconds *int) (Definition, error) {
	for _, definition := range definitions {
		if definition.View != view {
			continue
		}
		if view != "cpu" {
			if cpuRateWindowSeconds != nil {
				return Definition{}, errors.New("CPU rate window is only valid for the CPU view")
			}
			return checked(definition)
		}
		if cpuRateWindowSeconds == nil || !validCPUWindow(*cpuRateWindowSeconds) {
			return Definition{}, errors.New("CPU view requires a registered rate window")
		}
		definition.CanonicalExpression = cpuExpression(*cpuRateWindowSeconds)
		return checked(definition)
	}
	return Definition{}, errors.New("view is outside the node_exporter registry")
}

func cpuExpression(seconds int) string {
	window := fmt.Sprintf("%ds", seconds)
	if seconds%60 == 0 {
		window = fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf(`100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[%s])))`, window)
}

func validCPUWindow(value int) bool {
	for _, allowed := range cpuWindows {
		if value == allowed {
			return true
		}
	}
	return false
}

func checked(definition Definition) (Definition, error) {
	if _, err := normalizedExpression(definition.CanonicalExpression); err != nil {
		return Definition{}, errors.New("registry is invalid")
	}
	return definition, nil
}

// Validate accepts only one of the three registry expressions after PromQL
// parsing and normalization. This deliberately permits whitespace and
// redundant parentheses, but no semantic variation.
func Validate(expression string) (Definition, error) {
	normalized, err := normalizedExpression(expression)
	if err != nil {
		return Definition{}, err
	}
	candidates := append([]Definition(nil), definitions[1:]...)
	for _, window := range cpuWindows {
		definition, err := Resolve("cpu", &window)
		if err != nil {
			return Definition{}, errors.New("registry is invalid")
		}
		candidates = append(candidates, definition)
	}
	for _, definition := range candidates {
		canonical, err := normalizedExpression(definition.CanonicalExpression)
		if err != nil {
			return Definition{}, errors.New("registry is invalid")
		}
		if normalized == canonical {
			return definition, nil
		}
	}
	return Definition{}, errors.New("expression is outside the node_exporter registry")
}

func normalizedExpression(expression string) (string, error) {
	if utf8.RuneCountInString(expression) > MaxExpressionLength || strings.TrimSpace(expression) == "" {
		return "", errors.New("expression is empty or exceeds the length limit")
	}
	expression = strings.TrimSpace(expression)
	expr, err := parser.NewParser(parser.Options{}).ParseExpr(expression)
	if err != nil {
		return "", errors.New("expression is not valid PromQL")
	}
	nodes, selectors := 0, 0
	parser.Inspect(expr, func(node parser.Node, _ []parser.Node) error {
		nodes++
		if _, ok := node.(*parser.VectorSelector); ok {
			selectors++
		}
		return nil
	})
	if nodes > MaxASTNodes || selectors > MaxSelectors {
		return "", errors.New("expression exceeds the AST policy limits")
	}
	return stripOuterParens(expr).String(), nil
}

func stripOuterParens(expr parser.Expr) parser.Expr {
	for {
		parenthesized, ok := expr.(*parser.ParenExpr)
		if !ok {
			return expr
		}
		expr = parenthesized.Expr
	}
}
