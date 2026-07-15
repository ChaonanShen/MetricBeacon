package incident

import (
	"testing"
	"time"
)

func TestAlertEventStoresOnlyBoundedSummary(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	labels := map[string]string{"alertname": "OrderQueueBacklog", "service_ref": "order-demo"}
	value, err := NewAlertEvent("alert_1", AlertKey{TenantID: "org:1", OrgID: "1", SourceID: "demo-grafana", Fingerprint: "fp_1", StartsAt: now, Status: AlertFiring}, "order-demo", "OrderQueueBacklog", labels, "task_1", now)
	if err != nil {
		t.Fatal(err)
	}
	labels["ground_truth"] = "worker-stopped"
	if _, exists := value.Labels["ground_truth"]; exists {
		t.Fatal("AlertEvent retained caller map mutation")
	}
	if _, err := NewAlertEvent("alert_2", value.Key, value.ServiceRef, value.AlertName, value.Labels, "", now); err == nil {
		t.Fatal("firing alert without Task unexpectedly succeeded")
	}
}
