package common

import "time"

// TimeBounds describes the first and last returned sample. A single sample has
// equal bounds, unlike an AbsoluteTimeRange used for an executable query.
type TimeBounds struct {
	From time.Time
	To   time.Time
}

func NewTimeBounds(from, to time.Time) (TimeBounds, error) {
	from, to = from.UTC(), to.UTC()
	if from.IsZero() || to.IsZero() || from.After(to) {
		return TimeBounds{}, NewError(InvalidArgument, "time bounds are invalid", false)
	}
	return TimeBounds{From: from, To: to}, nil
}
