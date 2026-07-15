package order

import (
	"errors"
	"testing"
	"time"
)

func TestOrderStateMachine(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	o := New("o-1", "DEMO", 1, now)
	if o.Status != StatusQueued {
		t.Fatalf("status = %s", o.Status)
	}
	if err := o.Complete(now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("complete queued error = %v", err)
	}
	if err := o.Start(now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := o.Complete(now.Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := o.Fail("dependency_unavailable", now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("fail completed error = %v", err)
	}
}
