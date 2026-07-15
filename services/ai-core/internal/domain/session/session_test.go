package session

import (
	"errors"
	"testing"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

func TestAnalysisSessionTouchAdvancesActivityAndVersion(t *testing.T) {
	createdAt := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	value, err := NewPrivate("session_1", "org:1", "1", "CPU analysis", "user:1", createdAt)
	if err != nil {
		t.Fatal(err)
	}
	next := createdAt.Add(time.Minute).In(time.FixedZone("offset", 8*60*60))
	if err := value.Touch(next); err != nil {
		t.Fatal(err)
	}
	if value.UpdatedAt != next.UTC() || value.Version != 2 || value.Title != "CPU analysis" || value.CreatedBy != "user:1" || value.CreatedAt != createdAt {
		t.Fatalf("unexpected touched session: %#v", value)
	}
	if err := value.Touch(value.UpdatedAt); err != nil || value.Version != 3 {
		t.Fatalf("equal-time touch failed: %#v, %v", value, err)
	}
}

func TestAnalysisSessionTouchRejectsClockRegression(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	value, err := NewPrivate("session_1", "org:1", "1", "CPU analysis", "user:1", now)
	if err != nil {
		t.Fatal(err)
	}
	err = value.Touch(now.Add(-time.Nanosecond))
	var domainErr *common.DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != common.InvalidStateTransition || value.Version != 1 || value.UpdatedAt != now {
		t.Fatalf("unexpected regression result: %#v, %v", value, err)
	}
}

func TestSessionKindsRequireOrganizationAndStayExplicit(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	private, err := NewPrivate("private_1", "org:1", "1", "Private", "user:1", now)
	if err != nil || private.Kind != KindPrivate || private.OrgID != "1" {
		t.Fatalf("private Session = %#v, %v", private, err)
	}
	incident, err := NewIncident("incident_1", "org:1", "1", "Order backlog", "system:grafana", now)
	if err != nil || incident.Kind != KindOrgIncident || incident.CreatedBy != "system:grafana" {
		t.Fatalf("Incident Session = %#v, %v", incident, err)
	}
	if _, err := NewIncident("incident_2", "org:1", "", "Order backlog", "system:grafana", now); err == nil {
		t.Fatal("Incident Session without org unexpectedly succeeded")
	}
}
