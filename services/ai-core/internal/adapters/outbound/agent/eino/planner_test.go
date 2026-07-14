package eino_test

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/eino"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/profile"
	"mini-torchbearing.local/services/ai-core/internal/application/dto"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
)

func TestPlannerReturnsStrictBoundedIntentAndIncludesHistory(t *testing.T) {
	model := &scriptedModel{responses: []*schema.Message{schema.AssistantMessage(`{"status":"planned","views":["cpu"],"rangeSeconds":60,"stepSeconds":5}`, nil)}}
	planner := newPlanner(t, model)
	plan, err := planner.Plan(context.Background(), identity(), agent.IntentPlanRequest{Message: "那改成一分钟", History: []dto.ConversationMessage{{Role: "user", Content: "只看 CPU"}}})
	if err != nil || plan.Status != agent.IntentPlanned || len(plan.Views) != 1 || plan.Views[0] != "cpu" || plan.RangeDuration == nil || *plan.RangeDuration != time.Minute || plan.StepSeconds == nil || *plan.StepSeconds != 5 {
		t.Fatalf("plan = %#v, err = %v", plan, err)
	}
	if len(model.inputs) != 1 || len(model.inputs[0]) < 3 || model.inputs[0][1].Content != "只看 CPU" {
		t.Fatalf("history was not passed by message boundary: %#v", model.inputs)
	}
}

func TestPlannerRejectsUnknownDuplicateAndExtraJSON(t *testing.T) {
	responses := []string{
		`{"status":"planned","views":["disk"],"rangeSeconds":null,"stepSeconds":null}`,
		`{"status":"planned","views":["cpu","cpu"],"rangeSeconds":null,"stepSeconds":null}`,
		`{"status":"planned","views":["cpu"],"rangeSeconds":null,"stepSeconds":null,"promql":"up"}`,
	}
	for _, response := range responses {
		planner := newPlanner(t, &scriptedModel{responses: []*schema.Message{schema.AssistantMessage(response, nil)}})
		_, err := planner.Plan(context.Background(), identity(), agent.IntentPlanRequest{Message: "CPU"})
		assertCode(t, err, common.DependencyUnavailable)
	}
}

func newPlanner(t *testing.T, model *scriptedModel) *eino.Planner {
	t.Helper()
	nodeProfile, err := profile.Load(repositoryPath(t, "data/agent-knowledge/node_exporter.md"))
	if err != nil {
		t.Fatal(err)
	}
	planner, err := eino.NewPlanner(model, nodeProfile, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return planner
}
