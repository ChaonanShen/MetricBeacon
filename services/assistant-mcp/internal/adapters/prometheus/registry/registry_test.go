package registry

import (
	"strings"
	"testing"
)

func TestValidateAllowsOnlyCanonicalNodeExporterSemantics(t *testing.T) {
	for _, definition := range Definitions() {
		got, err := Validate(" ( " + definition.CanonicalExpression + " ) ")
		if err != nil || got.View != definition.View || got.CanonicalExpression != definition.CanonicalExpression {
			t.Fatalf("validate %q: %#v, %v", definition.View, got, err)
		}
	}
}

func TestValidateRejectsPromQLOutsideTheRegistry(t *testing.T) {
	for _, expression := range []string{
		"up",
		`node_load1{instance=~".*"}`,
		"sum(node_load1)",
		"node_load1 offset 5m",
		"node_load1 @ 1",
		"rate(node_load1[5m])",
		"node_cpu_seconds_total{mode=\"user\"}",
	} {
		if _, err := Validate(expression); err == nil {
			t.Fatalf("expression %q unexpectedly passed", expression)
		}
	}
}

func TestResolveRendersOnlyBoundedViewParameters(t *testing.T) {
	for _, test := range []struct {
		window int
		want   string
	}{{30, "[30s]"}, {60, "[1m]"}, {300, "[5m]"}} {
		definition, err := Resolve("cpu", &test.window)
		if err != nil || !strings.Contains(definition.CanonicalExpression, test.want) {
			t.Fatalf("window %d resolved to %#v, %v", test.window, definition, err)
		}
		validated, err := Validate(definition.CanonicalExpression)
		if err != nil || validated.View != "cpu" {
			t.Fatalf("rendered expression was not accepted internally: %#v, %v", validated, err)
		}
	}
	for _, invalid := range []struct {
		view   string
		window *int
	}{
		{view: "cpu"},
		{view: "memory", window: pointer(30)},
		{view: "unknown"},
	} {
		if _, err := Resolve(invalid.view, invalid.window); err == nil {
			t.Fatalf("invalid registry parameters were accepted: %#v", invalid)
		}
	}
}

func pointer(value int) *int { return &value }
