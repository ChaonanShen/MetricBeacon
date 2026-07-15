package order

import (
	"errors"
	"time"
)

type Status string

const (
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

var ErrInvalidTransition = errors.New("invalid order state transition")

type Order struct {
	ID            string
	SKU           string
	Quantity      int
	Status        Status
	FailureReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func New(id, sku string, quantity int, now time.Time) Order {
	return Order{ID: id, SKU: sku, Quantity: quantity, Status: StatusQueued, CreatedAt: now, UpdatedAt: now}
}

func (o *Order) Start(now time.Time) error {
	if o.Status != StatusQueued {
		return ErrInvalidTransition
	}
	o.Status = StatusProcessing
	o.UpdatedAt = now
	return nil
}

func (o *Order) Complete(now time.Time) error {
	if o.Status != StatusProcessing {
		return ErrInvalidTransition
	}
	o.Status = StatusCompleted
	o.FailureReason = ""
	o.UpdatedAt = now
	return nil
}

func (o *Order) Fail(reason string, now time.Time) error {
	if o.Status != StatusProcessing {
		return ErrInvalidTransition
	}
	o.Status = StatusFailed
	o.FailureReason = reason
	o.UpdatedAt = now
	return nil
}
