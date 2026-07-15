package incident

import (
	"context"
	"encoding/json"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
)

type ToolEvidence struct {
	Name          string
	InputSummary  json.RawMessage
	OutputSummary json.RawMessage
	DurationMS    int64
}

type ResolvedRun struct {
	MappingID, MappingDigest                    string
	PlaybookID, PlaybookVersion, PlaybookDigest string
	ServiceRef, Checkpoint                      string
	AssetRefs                                   []task.AssetRef
	Evidence                                    []ToolEvidence
}

type Observation struct {
	Diagnosis task.Diagnosis
	Evidence  []ToolEvidence
}

type Toolset interface {
	ResolveAndStart(context.Context, requestcontext.Context, string, string, map[string]string) (ResolvedRun, error)
	Observe(context.Context, requestcontext.Context, string) (Observation, error)
}
