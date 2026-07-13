package testkit

import "time"

// FixedClock is a deterministic clock suitable for domain and adapter tests.
type FixedClock struct{ Value time.Time }

func (c FixedClock) Now() time.Time { return c.Value.UTC() }
