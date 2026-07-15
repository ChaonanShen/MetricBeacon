package orderdemo

import (
	"context"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
)

// Port exposes only bounded, typed operational reads. It intentionally has no
// fault-injection or generic HTTP/command method.
type Port interface {
	GetRuntime(context.Context, requestcontext.Context) (Runtime, error)
	GetQueue(context.Context, requestcontext.Context) (Queue, error)
	GetWorker(context.Context, requestcontext.Context) (Worker, error)
	GetPolicy(context.Context, requestcontext.Context) (Policy, error)
	GetRecentOutcomes(context.Context, requestcontext.Context, RecentRequest) ([]OrderOutcome, error)
	GetOperation(context.Context, requestcontext.Context, string) (Operation, error)
}
