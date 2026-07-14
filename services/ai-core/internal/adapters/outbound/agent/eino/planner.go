package eino

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/profile"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
)

const plannerProtocol = `

You are a bounded query intent planner. Return exactly one JSON object, without Markdown or prose:
{"status":"planned","views":["cpu"],"rangeSeconds":60,"stepSeconds":5}

status is planned or unsupported. views may contain each of cpu, memory, load at most once and in the order requested. rangeSeconds and stepSeconds must be JSON null when omitted. For unsupported requests return status unsupported, empty views, and null rangeSeconds/stepSeconds. Never output PromQL, datasource, absolute timestamps, CPU windows, labels, reasoning, metric values, identities, URLs, or secrets.`

type Planner struct {
	model   model.ToolCallingChatModel
	profile profile.Profile
	timeout time.Duration
}

var _ agent.IntentPlanner = (*Planner)(nil)

func NewPlanner(chatModel model.ToolCallingChatModel, nodeProfile profile.Profile, timeout time.Duration) (*Planner, error) {
	if chatModel == nil {
		return nil, common.NewError(common.AdapterNotConfigured, "intent planner model is required", false)
	}
	if err := nodeProfile.Validate(); err != nil {
		return nil, err
	}
	return &Planner{model: chatModel, profile: nodeProfile, timeout: timeout}, nil
}

func (p *Planner) Plan(ctx context.Context, _ requestcontext.Context, request agent.IntentPlanRequest) (agent.IntentPlan, error) {
	if strings.TrimSpace(request.Message) == "" {
		return agent.IntentPlan{}, common.NewError(common.InvalidArgument, "planner message is required", false)
	}
	if p.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}
	messages := []*schema.Message{schema.SystemMessage(p.profile.Content + plannerProtocol)}
	for _, historical := range request.History {
		if historical.Role == "assistant" {
			messages = append(messages, schema.AssistantMessage(historical.Content, nil))
		} else if historical.Role == "user" {
			messages = append(messages, schema.UserMessage(historical.Content))
		}
	}
	messages = append(messages, schema.UserMessage(request.Message))
	response, err := p.model.Generate(ctx, messages)
	if err != nil {
		return agent.IntentPlan{}, common.NewError(common.DependencyUnavailable, "query intent planner is unavailable", true)
	}
	if response == nil || response.Role != schema.Assistant {
		return agent.IntentPlan{}, common.NewError(common.DependencyUnavailable, "query intent planner returned an unreadable response", true)
	}
	var wire struct {
		Status       string   `json:"status"`
		Views        []string `json:"views"`
		RangeSeconds *int     `json:"rangeSeconds"`
		StepSeconds  *int     `json:"stepSeconds"`
	}
	if err := decodeStrict(strings.TrimSpace(response.Content), &wire); err != nil {
		return agent.IntentPlan{}, common.NewError(common.DependencyUnavailable, "query intent planner returned invalid JSON", true)
	}
	plan := agent.IntentPlan{Status: agent.IntentStatus(wire.Status), Views: wire.Views, StepSeconds: wire.StepSeconds}
	if wire.RangeSeconds != nil {
		duration := time.Duration(*wire.RangeSeconds) * time.Second
		plan.RangeDuration = &duration
	}
	if err := validateIntentPlan(plan); err != nil {
		return agent.IntentPlan{}, err
	}
	return plan, nil
}

func validateIntentPlan(plan agent.IntentPlan) error {
	if plan.Status == agent.IntentUnsupported {
		if len(plan.Views) != 0 || plan.RangeDuration != nil || plan.StepSeconds != nil {
			return common.NewError(common.DependencyUnavailable, "query intent planner returned inconsistent unsupported intent", true)
		}
		return nil
	}
	if plan.Status != agent.IntentPlanned || len(plan.Views) == 0 {
		return common.NewError(common.DependencyUnavailable, "query intent planner returned incomplete intent", true)
	}
	seen := map[string]struct{}{}
	for _, view := range plan.Views {
		if view != "cpu" && view != "memory" && view != "load" {
			return common.NewError(common.DependencyUnavailable, "query intent planner returned an unknown view", true)
		}
		if _, exists := seen[view]; exists {
			return common.NewError(common.DependencyUnavailable, "query intent planner returned duplicate views", true)
		}
		seen[view] = struct{}{}
	}
	return nil
}
