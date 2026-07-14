package task

import (
	"testing"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

func TestTaskStateMachine(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC)
	range30m, err := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	if err != nil {
		t.Fatal(err)
	}
	task, err := New("task_1", "org:1", "session_1", "message_1", "prometheus-main", range30m, LegacyQueryPlan(), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, next := range []Status{StatusPlanning, StatusRunningTools, StatusValidating, StatusCompleted} {
		if err := task.Transition(next, now); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
	if task.StartedAt == nil || task.CompletedAt == nil {
		t.Fatal("terminal task must have timestamps")
	}
	if err := task.Transition(StatusFailed, now); err == nil {
		t.Fatal("terminal task transition unexpectedly succeeded")
	}
}

func TestTaskFailureFromNonTerminal(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC)
	range30m, _ := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	task, _ := New("task_1", "org:1", "session_1", "message_1", "prometheus-main", range30m, LegacyQueryPlan(), now)
	err := task.Fail(common.NewError(common.DependencyUnavailable, "assistant-mcp is unavailable", true), now)
	if err != nil || task.Status != StatusFailed || task.Error == nil {
		t.Fatalf("unexpected failed task: %#v, %v", task, err)
	}
}
