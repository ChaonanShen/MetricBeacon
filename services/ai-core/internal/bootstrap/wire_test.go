package bootstrap

import (
	"context"
	"testing"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/ports/tools"
)

func TestIncidentRemediationReadinessUsesContractNamespacesAndExactToolCounts(t *testing.T) {
	gateway := &readinessGateway{counts: map[string]int{"knowledge": 1, "skills": 1, "playbook": 3, "order_service": 9}, seen: map[string]int{}}
	identity := requestcontext.Context{TenantID: "org:1", OrgID: "1", UserID: "readiness", Permissions: []string{"incidents:diagnose", "incidents:remediate"}}
	if err := validateIncidentRemediationProfile(context.Background(), gateway, identity); err != nil {
		t.Fatal(err)
	}
	for _, namespace := range []string{"knowledge", "skills", "playbook", "order_service"} {
		if gateway.seen[namespace] != 1 {
			t.Fatalf("namespace %q was checked %d times", namespace, gateway.seen[namespace])
		}
	}
	if gateway.seen["skill"] != 0 {
		t.Fatal("readiness used the non-contract singular Skill namespace")
	}
}

func TestIncidentRemediationReadinessFailsClosedOnMissingTool(t *testing.T) {
	gateway := &readinessGateway{counts: map[string]int{"knowledge": 1, "skills": 1, "playbook": 3, "order_service": 8}, seen: map[string]int{}}
	if err := validateIncidentRemediationProfile(context.Background(), gateway, requestcontext.Context{}); err == nil {
		t.Fatal("incomplete remediation profile passed readiness")
	}
}

type readinessGateway struct {
	counts map[string]int
	seen   map[string]int
}

func (g *readinessGateway) ListTools(_ context.Context, _ requestcontext.Context, filter tools.Filter) ([]tools.Descriptor, error) {
	g.seen[filter.Namespace]++
	return make([]tools.Descriptor, g.counts[filter.Namespace]), nil
}

func (*readinessGateway) CallTool(context.Context, requestcontext.Context, tools.Call) (tools.Result, error) {
	return tools.Result{}, nil
}
