package tools

import (
	"context"

	"mini-torchbearing.local/packages/request-context-go"
)

type Filter struct{ Namespace string }
type Descriptor struct{ Name, Version string }
type Call struct {
	Name, Version string
	Arguments     []byte
}
type Result struct{ Content []byte }
type Gateway interface {
	ListTools(context.Context, requestcontext.Context, Filter) ([]Descriptor, error)
	CallTool(context.Context, requestcontext.Context, Call) (Result, error)
}
