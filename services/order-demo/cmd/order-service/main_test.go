package main

import "testing"

func TestEnvOr(t *testing.T) {
	t.Setenv("ORDER_DEMO_TEST_VALUE", "configured")
	if got := envOr("ORDER_DEMO_TEST_VALUE", "fallback"); got != "configured" {
		t.Fatalf("value = %q", got)
	}
	if got := envOr("ORDER_DEMO_TEST_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("fallback = %q", got)
	}
}
