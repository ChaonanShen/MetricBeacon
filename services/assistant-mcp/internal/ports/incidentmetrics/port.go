package incidentmetrics

import (
	"context"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
)

type Recovery struct {
	WindowSeconds    int
	AcceptedDelta    float64
	CompletedDelta   float64
	QueueDepth       float64
	OldestAgeSeconds float64
	ObservedAt       time.Time
}

// Port exposes one registered recovery view. Callers cannot provide PromQL,
// metric names, time ranges, endpoints or arbitrary query parameters.
type Port interface {
	GetRecovery(context.Context, requestcontext.Context) (Recovery, error)
}
