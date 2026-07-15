package metrics

import "time"

type Recorder interface {
	OrderReceived(result string)
	OrderCompleted(result string)
	OrderRetried(reason string)
	OrderFailed(reason string)
	ObserveProcessing(time.Duration)
	ObserveEndToEnd(time.Duration)
	BusinessProbe(result string)
}

type Noop struct{}

func (Noop) OrderReceived(string)            {}
func (Noop) OrderCompleted(string)           {}
func (Noop) OrderRetried(string)             {}
func (Noop) OrderFailed(string)              {}
func (Noop) ObserveProcessing(time.Duration) {}
func (Noop) ObserveEndToEnd(time.Duration)   {}
func (Noop) BusinessProbe(string)            {}
