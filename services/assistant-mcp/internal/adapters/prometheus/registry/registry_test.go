package registry

import "testing"

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
