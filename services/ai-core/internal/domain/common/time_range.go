package common

import "time"

type AbsoluteTimeRange struct {
	From time.Time
	To   time.Time
}

func NewAbsoluteTimeRange(from, to time.Time) (AbsoluteTimeRange, error) {
	from, to = from.UTC(), to.UTC()
	if !from.Before(to) {
		return AbsoluteTimeRange{}, NewError(InvalidArgument, "time range from must be before to", false)
	}
	return AbsoluteTimeRange{From: from, To: to}, nil
}
