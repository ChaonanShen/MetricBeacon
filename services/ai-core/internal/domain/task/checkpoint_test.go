package task

import (
	"strings"
	"testing"
	"time"
)

func TestCheckpointIsOpaqueBoundedAndVersioned(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	value, err := NewCheckpoint("task_1", "org:1", PhaseNeedsAgent, "signed:checkpoint", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := value.Replace(PhasePrepare, "signed:next", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if value.Version != 2 || value.Phase != PhasePrepare || value.OpaqueValue != "signed:next" {
		t.Fatalf("checkpoint = %#v", value)
	}
	if _, err := NewCheckpoint("task_2", "org:1", PhaseNeedsAgent, strings.Repeat("x", maxCheckpointBytes+1), now); err == nil {
		t.Fatal("oversized checkpoint unexpectedly succeeded")
	}
	if err := value.Replace(PhasePrepare, "stale", now.Add(-time.Second)); err == nil {
		t.Fatal("checkpoint clock regression unexpectedly succeeded")
	}
}
