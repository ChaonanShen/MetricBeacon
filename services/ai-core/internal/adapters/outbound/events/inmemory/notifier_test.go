package inmemory

import (
	"context"
	"testing"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/task"
)

func TestNotifierSignalsOnlyMatchingSubscribers(t *testing.T) {
	notifier := New()
	matching, err := notifier.Subscribe(context.Background(), "org:1", "task_1")
	if err != nil {
		t.Fatal(err)
	}
	nonMatching, err := notifier.Subscribe(context.Background(), "org:1", "task_2")
	if err != nil {
		t.Fatal(err)
	}
	if err := notifier.Notify(context.Background(), task.TaskEvent{EventDraft: task.EventDraft{TenantID: "org:1", TaskID: "task_1"}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-matching:
	case <-time.After(time.Second):
		t.Fatal("matching subscriber was not notified")
	}
	select {
	case <-nonMatching:
		t.Fatal("non-matching subscriber was notified")
	default:
	}
}
