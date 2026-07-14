// Package registry owns the narrowly-scoped node_exporter query allowlist.
// It belongs to the Prometheus adapter boundary so parser types never reach
// MCP handlers, ports, or AI Core.
package registry

import (
	"errors"
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
	{View: "cpu", Title: "CPU 使用率", Unit: "percent", RefID: "A", CanonicalExpression: `100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`, MetricNames: []string{"node_cpu_seconds_total"}, LabelNames: []string{"instance", "mode"}},
	{View: "memory", Title: "内存可用率", Unit: "percent", RefID: "B", CanonicalExpression: `100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes`, MetricNames: []string{"node_memory_MemAvailable_bytes", "node_memory_MemTotal_bytes"}, LabelNames: []string{"instance"}},
	{View: "load", Title: "系统负载", Unit: "short", RefID: "C", CanonicalExpression: `node_load1`, MetricNames: []string{"node_load1"}, LabelNames: []string{"instance"}},
}

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

// Validate accepts only one of the three registry expressions after PromQL
// parsing and normalization. This deliberately permits whitespace and
// redundant parentheses, but no semantic variation.
func Validate(expression string) (Definition, error) {
	if utf8.RuneCountInString(expression) > MaxExpressionLength || strings.TrimSpace(expression) == "" {
		return Definition{}, errors.New("expression is empty or exceeds the length limit")
	}
	expression = strings.TrimSpace(expression)
	expr, err := parser.NewParser(parser.Options{}).ParseExpr(expression)
	if err != nil {
		return Definition{}, errors.New("expression is not valid PromQL")
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
		return Definition{}, errors.New("expression exceeds the AST policy limits")
	}
	normalized := stripOuterParens(expr).String()
	for _, definition := range definitions {
		canonical, err := parser.NewParser(parser.Options{}).ParseExpr(definition.CanonicalExpression)
		if err != nil {
			return Definition{}, errors.New("registry is invalid")
		}
		if normalized == stripOuterParens(canonical).String() {
			return definition, nil
		}
	}
	return Definition{}, errors.New("expression is outside the node_exporter registry")
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
