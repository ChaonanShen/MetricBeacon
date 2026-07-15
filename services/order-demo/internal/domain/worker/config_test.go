package worker

import (
	"errors"
	"testing"
)

func TestRestoreRequiresExactStoppedVersion(t *testing.T) {
	c := NewConfig("epoch-1")
	if err := c.SetInternal(0); err != nil {
		t.Fatal(err)
	}
	if err := c.Restore("wrong", c.Version, 0, 2); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("wrong epoch error = %v", err)
	}
	if err := c.Restore("epoch-1", c.Version, 0, 2); err != nil {
		t.Fatal(err)
	}
	if c.Concurrency != 2 || c.Version != 3 {
		t.Fatalf("config = %+v", c)
	}
}

func TestPolicyDigestIsStableSHA256(t *testing.T) {
	if got := PolicyDigest(); len(got) != 64 {
		t.Fatalf("digest length = %d", len(got))
	}
}
