package session

import (
	"errors"
	"testing"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

func TestAnalysisSessionTouchAdvancesActivityAndVersion(t *testing.T) {
	createdAt := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	value, err := New("session_1", "org:1", "CPU analysis", "user:1", createdAt)
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
	value, err := New("session_1", "org:1", "CPU analysis", "user:1", now)
	if err != nil {
		t.Fatal(err)
	}
	err = value.Touch(now.Add(-time.Nanosecond))
	var domainErr *common.DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != common.InvalidStateTransition || value.Version != 1 || value.UpdatedAt != now {
		t.Fatalf("unexpected regression result: %#v, %v", value, err)
	}
}
