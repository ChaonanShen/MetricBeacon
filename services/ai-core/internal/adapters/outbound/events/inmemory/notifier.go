// Package inmemory provides the process-local wake-up notifier used by SSE.
package inmemory

import (
	"context"
	"sync"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
)

// Notifier is deliberately not an event source. Subscribers must replay the
// durable TaskEventStore after every signal.
type Notifier struct {
	mu          sync.Mutex
	subscribers map[string][]chan struct{}
}

func New() *Notifier { return &Notifier{subscribers: make(map[string][]chan struct{})} }

func (n *Notifier) Notify(_ context.Context, event task.TaskEvent) error {
	if event.TenantID == "" || event.TaskID == "" {
		return common.NewError(common.InvalidArgument, "event tenant and task are required for notification", false)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, subscriber := range n.subscribers[notificationKey(event.TenantID, event.TaskID)] {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
	return nil
}

func (n *Notifier) Subscribe(_ context.Context, tenantID, taskID string) (<-chan struct{}, error) {
	if tenantID == "" || taskID == "" {
		return nil, common.NewError(common.InvalidArgument, "tenant and task are required for subscription", false)
	}
	channel := make(chan struct{}, 1)
	n.mu.Lock()
	n.subscribers[notificationKey(tenantID, taskID)] = append(n.subscribers[notificationKey(tenantID, taskID)], channel)
	n.mu.Unlock()
	return channel, nil
}

func notificationKey(tenantID, taskID string) string { return tenantID + "\x00" + taskID }
