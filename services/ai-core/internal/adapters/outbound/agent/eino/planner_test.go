package eino_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/eino"
	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/profile"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
)

func TestPlannerReturnsStrictBoundedIntentAndIncludesHistory(t *testing.T) {
	model := &scriptedModel{responses: []*schema.Message{schema.AssistantMessage(`{"status":"planned","views":["cpu"],"rangeSeconds":60,"stepSeconds":5}`, nil)}}
	planner := newPlanner(t, model)
	plan, err := planner.Plan(context.Background(), identity(), agent.IntentPlanRequest{Message: "那改成一分钟", PreviousIntents: []agent.IntentHistoryItem{{Message: "只看 CPU", Views: []string{"cpu"}, RangeSeconds: 1800, StepSeconds: 10}}})
	if err != nil || plan.Status != agent.IntentPlanned || len(plan.Views) != 1 || plan.Views[0] != "cpu" || plan.RangeDuration == nil || *plan.RangeDuration != time.Minute || plan.StepSeconds == nil || *plan.StepSeconds != 5 {
		t.Fatalf("plan = %#v, err = %v", plan, err)
	}
	if len(model.inputs) != 1 || len(model.inputs[0]) != 2 || model.inputs[0][0].Role != schema.System || model.inputs[0][1].Role != schema.User {
		t.Fatalf("planner did not use one system and one JSON input message: %#v", model.inputs)
	}
	if strings.Contains(model.inputs[0][0].Content, "node_cpu_seconds_total") || strings.Contains(model.inputs[0][0].Content, "最终回复格式") {
		t.Fatalf("execution profile leaked into planner system prompt: %s", model.inputs[0][0].Content)
	}
	var envelope struct {
		PreviousIntents []struct {
			Message      string   `json:"message"`
			Views        []string `json:"views"`
			RangeSeconds int      `json:"rangeSeconds"`
			StepSeconds  int      `json:"stepSeconds"`
		} `json:"previousIntents"`
		CurrentMessage string `json:"currentMessage"`
	}
	if err := json.Unmarshal([]byte(model.inputs[0][1].Content), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.CurrentMessage != "那改成一分钟" || len(envelope.PreviousIntents) != 1 || envelope.PreviousIntents[0].Message != "只看 CPU" || envelope.PreviousIntents[0].RangeSeconds != 1800 || envelope.PreviousIntents[0].StepSeconds != 10 {
		t.Fatalf("unexpected planner envelope: %#v", envelope)
	}
}

func TestPlannerRetriesModelContractFailureOnce(t *testing.T) {
	model := &scriptedModel{responses: []*schema.Message{
		schema.AssistantMessage("已查询 node_exporter", nil),
		schema.AssistantMessage(`{"status":"planned","views":["cpu","memory"],"rangeSeconds":600,"stepSeconds":120}`, nil),
	}}
	planner := newPlanner(t, model)
	plan, err := planner.Plan(context.Background(), identity(), agent.IntentPlanRequest{Message: "查看最近10min的cpu和内存变化图，每隔2min采集一个数据点"})
	if err != nil || len(plan.Views) != 2 || plan.RangeDuration == nil || *plan.RangeDuration != 10*time.Minute || plan.StepSeconds == nil || *plan.StepSeconds != 120 {
		t.Fatalf("plan = %#v, err = %v", plan, err)
	}
	if len(model.inputs) != 2 || !strings.Contains(model.inputs[1][0].Content, "previous attempt") {
		t.Fatalf("planner retry inputs = %#v", model.inputs)
	}
}

func TestPlannerRejectsInvalidJSONShapesAfterOneRetry(t *testing.T) {
	responses := []string{
		`{"status":"planned","views":["disk"],"rangeSeconds":null,"stepSeconds":null}`,
		`{"status":"planned","views":["cpu","cpu"],"rangeSeconds":null,"stepSeconds":null}`,
		`{"status":"planned","views":["cpu"],"rangeSeconds":null,"stepSeconds":null,"promql":"up"}`,
		`{"status":"planned","views":["cpu"],"rangeSeconds":null}`,
		`{"status":"planned","views":null,"rangeSeconds":null,"stepSeconds":null}`,
		`{"status":"planned","status":"unsupported","views":["cpu"],"rangeSeconds":null,"stepSeconds":null}`,
		`{"status":"unsupported","views":["cpu"],"rangeSeconds":null,"stepSeconds":null}`,
		"",
		"已查询 node_exporter",
	}
	for _, response := range responses {
		model := &scriptedModel{responses: []*schema.Message{schema.AssistantMessage(response, nil), schema.AssistantMessage(response, nil)}}
		planner := newPlanner(t, model)
		_, err := planner.Plan(context.Background(), identity(), agent.IntentPlanRequest{Message: "CPU"})
		assertCode(t, err, common.DependencyUnavailable)
		if len(model.inputs) != 2 {
			t.Fatalf("response %q generated %d calls, want 2", response, len(model.inputs))
		}
	}
}

func TestPlannerAcceptsCompleteUnsupportedJSON(t *testing.T) {
	planner := newPlanner(t, &scriptedModel{responses: []*schema.Message{schema.AssistantMessage(`{"status":"unsupported","views":[],"rangeSeconds":null,"stepSeconds":null}`, nil)}})
	plan, err := planner.Plan(context.Background(), identity(), agent.IntentPlanRequest{Message: "查看磁盘"})
	if err != nil || plan.Status != agent.IntentUnsupported || len(plan.Views) != 0 || plan.RangeDuration != nil || plan.StepSeconds != nil {
		t.Fatalf("plan = %#v, err = %v", plan, err)
	}
}

func TestPlannerDoesNotRetryModelTransportFailure(t *testing.T) {
	model := &scriptedModel{
		generateErrors: []error{errors.New("upstream unavailable")},
		responses:      []*schema.Message{schema.AssistantMessage(`{"status":"planned","views":["cpu"],"rangeSeconds":null,"stepSeconds":null}`, nil)},
	}
	planner := newPlanner(t, model)
	_, err := planner.Plan(context.Background(), identity(), agent.IntentPlanRequest{Message: "CPU"})
	assertCode(t, err, common.DependencyUnavailable)
	if len(model.inputs) != 1 {
		t.Fatalf("transport failure generated %d calls, want 1", len(model.inputs))
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
