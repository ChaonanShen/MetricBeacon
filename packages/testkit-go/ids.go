package testkit

import "fmt"

// SequenceIDGenerator is deterministic and intentionally satisfies ID ports structurally.
type SequenceIDGenerator struct {
	Prefix string
	Next   int
}

func (g *SequenceIDGenerator) NewID(kind string) string {
	g.Next++
	return fmt.Sprintf("%s%s_%03d", g.Prefix, kind, g.Next)
}
