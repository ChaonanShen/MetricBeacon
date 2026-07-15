package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

const (
	ExpectedConcurrency = 2
	MinConcurrency      = 1
	MaxConcurrency      = 4
)

var ErrPreconditionFailed = errors.New("worker configuration precondition failed")

type Config struct {
	InstanceEpoch string
	Concurrency   int
	Version       int
}

func NewConfig(instanceEpoch string) Config {
	return Config{InstanceEpoch: instanceEpoch, Concurrency: ExpectedConcurrency, Version: 1}
}

func (c *Config) SetInternal(concurrency int) error {
	if concurrency < 0 || concurrency > MaxConcurrency {
		return ErrPreconditionFailed
	}
	if c.Concurrency != concurrency {
		c.Concurrency = concurrency
		c.Version++
	}
	return nil
}

func (c *Config) Restore(instanceEpoch string, expectedVersion, expectedConcurrency, newConcurrency int) error {
	if instanceEpoch != c.InstanceEpoch || expectedVersion != c.Version || expectedConcurrency != c.Concurrency || expectedConcurrency != 0 || newConcurrency != ExpectedConcurrency {
		return ErrPreconditionFailed
	}
	c.Concurrency = newConcurrency
	c.Version++
	return nil
}

func PolicyDigest() string {
	sum := sha256.Sum256([]byte("order-demo-worker-policy:v1:expected=2:min=1:max=4"))
	return hex.EncodeToString(sum[:])
}
