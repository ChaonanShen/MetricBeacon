package mock

import (
	"context"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/incidentmetrics"
)

type Adapter struct{ now func() time.Time }

func New(now func() time.Time) *Adapter {
	if now == nil {
		now = time.Now
	}
	return &Adapter{now: now}
}

func (a *Adapter) GetRecovery(context.Context, requestcontext.Context) (incidentmetrics.Recovery, error) {
	return incidentmetrics.Recovery{WindowSeconds: 30, AcceptedDelta: 10, CompletedDelta: 10, QueueDepth: 0, OldestAgeSeconds: 0, ObservedAt: a.now().UTC()}, nil
}
