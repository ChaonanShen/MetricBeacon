// Package ids supplies process-local opaque identifiers for the mock runtime.
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// Generator intentionally exposes no storage or transport concerns. IDs are
// opaque to callers, but keep a useful prefix for logs and fixtures.
type Generator struct{}

func New() Generator { return Generator{} }

func (Generator) NewID(kind string) string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand failures are exceptionally rare. The zero value remains
		// opaque and avoids turning an observability concern into a panic.
		return strings.TrimSpace(kind) + "_00000000000000000000000000000000"
	}
	return strings.TrimSpace(kind) + "_" + hex.EncodeToString(bytes)
}
