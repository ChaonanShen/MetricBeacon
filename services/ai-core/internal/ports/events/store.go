package events

import (
	"context"

	"mini-torchbearing.local/services/ai-core/internal/domain/task"
)

type Store interface {
	Append(context.Context, task.EventDraft) (task.TaskEvent, error)
	Replay(context.Context, string, string, int64, int) ([]task.TaskEvent, error)
	LatestSequence(context.Context, string, string) (int64, error)
}

type Notifier interface {
	Notify(context.Context, task.TaskEvent) error
	Subscribe(context.Context, string, string) (<-chan struct{}, error)
}
