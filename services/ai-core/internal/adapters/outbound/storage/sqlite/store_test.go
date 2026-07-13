package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	storage "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/storage/sqlite"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
)

const tenantID = "org:1"

func TestApplicationStoreCRUDAndTenantIsolation(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)
	data := createTaskFixture(t, ctx, store)

	loadedSession, err := store.Sessions().Get(ctx, tenantID, data.session.ID)
	if err != nil {
		t.Fatal(err)
	}
	loadedSession.Title = "Updated node exporter analysis"
	loadedSession.UpdatedAt = data.now.Add(time.Minute)
	loadedSession.Version++
	if err := store.Sessions().Update(ctx, loadedSession, loadedSession.Version-1); err != nil {
		t.Fatal(err)
	}

	messages, err := store.Messages().ListBySession(ctx, tenantID, data.session.ID)
	if err != nil || len(messages) != 1 || messages[0].Content != data.message.Content {
		t.Fatalf("unexpected messages: %#v, %v", messages, err)
	}

	loadedTask, err := store.Tasks().Get(ctx, tenantID, data.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := loadedTask.Transition(task.StatusPlanning, data.now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.Tasks().Update(ctx, loadedTask, loadedTask.Version-1); err != nil {
		t.Fatal(err)
	}

	started := task.ToolCallRecord{
		ID:           "tool_1",
		TenantID:     tenantID,
		TaskID:       data.task.ID,
		ToolName:     "grafana.search_metrics",
		ToolVersion:  "v1",
		Status:       task.ToolCallStarted,
		InputSummary: []byte(`{"datasourceUid":"mock-prometheus"}`),
		StartedAt:    data.now,
		Version:      1,
	}
	if err := store.ToolCalls().Create(ctx, started); err != nil {
		t.Fatal(err)
	}
	duration := int64(15)
	completedAt := data.now.Add(15 * time.Millisecond)
	completed := started
	completed.Status = task.ToolCallCompleted
	completed.OutputSummary = []byte(`{"candidateCount":4}`)
	completed.CompletedAt = &completedAt
	completed.DurationMS = &duration
	completed.Version++
	if err := store.ToolCalls().Complete(ctx, completed, started.Version); err != nil {
		t.Fatal(err)
	}
	toolCalls, err := store.ToolCalls().ListByTask(ctx, tenantID, data.task.ID)
	if err != nil || len(toolCalls) != 1 || toolCalls[0].Status != task.ToolCallCompleted {
		t.Fatalf("unexpected tool calls: %#v, %v", toolCalls, err)
	}

	query := chart.QuerySpec{RefID: "A", Expression: "up", Legend: "{{instance}}", DatasourceUID: "mock-prometheus", TimeRange: data.timeRange}
	draft, err := chart.New("chart_1", tenantID, data.session.ID, data.task.ID, "CPU usage", "percent", []chart.QuerySpec{query}, data.now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Charts().Create(ctx, draft); err != nil {
		t.Fatal(err)
	}
	execution := chart.Execution{
		ID:          "execution_1",
		TenantID:    tenantID,
		ChartID:     draft.ID,
		QueryRefID:  "A",
		Status:      chart.ExecutionSuccess,
		DurationMS:  12,
		SampleRange: data.timeRange,
		Series: []chart.Series{{
			Name:   "node-a",
			Labels: map[string]string{"instance": "node-a"},
			Points: []chart.Point{{Timestamp: data.timeRange.From, Value: 12.5}},
		}},
		Warnings:  []string{},
		CreatedAt: data.now,
	}
	if err := store.ChartExecutions().Create(ctx, execution); err != nil {
		t.Fatal(err)
	}
	charts, err := store.Charts().ListByTask(ctx, tenantID, data.task.ID)
	if err != nil || len(charts) != 1 || charts[0].Title != draft.Title {
		t.Fatalf("unexpected charts: %#v, %v", charts, err)
	}
	executions, err := store.ChartExecutions().ListByChart(ctx, tenantID, draft.ID)
	if err != nil || len(executions) != 1 || len(executions[0].Series) != 1 {
		t.Fatalf("unexpected executions: %#v, %v", executions, err)
	}

	_, err = store.Sessions().Get(ctx, "org:other", data.session.ID)
	requireCode(t, err, common.ResourceNotFound)
	_, err = store.Tasks().Get(ctx, "org:other", data.task.ID)
	requireCode(t, err, common.ResourceNotFound)
	forged, err := session.NewMessage("message_forged", "org:other", data.session.ID, session.RoleUser, "forged", data.now)
	if err != nil {
		t.Fatal(err)
	}
	requireCode(t, store.Messages().Append(ctx, forged), common.ResourceNotFound)

	duplicate := data.session
	duplicate.Title = "duplicate"
	requireCode(t, store.Sessions().Create(ctx, duplicate), common.ResourceConflict)
}

func TestApplicationStoreRollbackAndIdempotencyContract(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)
	now := fixedNow()
	rolledBack, err := session.New("session_rollback", tenantID, "rolled back", "user:1", now)
	if err != nil {
		t.Fatal(err)
	}
	err = store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Sessions().Create(ctx, rolledBack); err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("transaction unexpectedly committed")
	}
	_, err = store.Sessions().Get(ctx, tenantID, rolledBack.ID)
	requireCode(t, err, common.ResourceNotFound)

	key := repositories.IdempotencyKey{TenantID: tenantID, Scope: "create_task", Key: "idem_1"}
	expiresAt := time.Now().UTC().Add(time.Hour)
	first, err := store.Idempotency().Reserve(ctx, key, "hash-one", expiresAt)
	if err != nil || first.Status != "reserved" {
		t.Fatalf("unexpected first reservation: %#v, %v", first, err)
	}
	retry, err := store.Idempotency().Reserve(ctx, key, "hash-one", expiresAt)
	if err != nil || retry.RequestHash != "hash-one" || retry.Status != "reserved" {
		t.Fatalf("unexpected retry reservation: %#v, %v", retry, err)
	}
	if err := store.Idempotency().Complete(ctx, key, "task_1", []byte(`{"id":"task_1"}`)); err != nil {
		t.Fatal(err)
	}
	completed, err := store.Idempotency().GetResult(ctx, key)
	if err != nil || completed.Status != "completed" || completed.ResourceID != "task_1" {
		t.Fatalf("unexpected completed idempotency record: %#v, %v", completed, err)
	}
	_, err = store.Idempotency().Reserve(ctx, key, "hash-two", expiresAt)
	requireCode(t, err, common.IdempotencyConflict)
}

func TestTaskEventsAreAtomicSequentialAndReplayable(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)
	data := createTaskFixture(t, ctx, store)

	for i := 1; i <= 203; i++ {
		index := i
		err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
			event, err := tx.TaskEvents().Append(ctx, task.EventDraft{
				EventID:   "event_" + fmt.Sprint(index),
				TenantID:  tenantID,
				TaskID:    data.task.ID,
				SessionID: data.session.ID,
				Type:      task.EventTaskStatusChanged,
				Timestamp: data.now.Add(time.Duration(index) * time.Second),
				Payload:   []byte(`{"previousStatus":"created","status":"planning"}`),
			})
			if err != nil {
				return err
			}
			if event.Sequence != int64(index) {
				return fmt.Errorf("got sequence %d, want %d", event.Sequence, index)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	latest, err := store.TaskEvents().LatestSequence(ctx, tenantID, data.task.ID)
	if err != nil || latest != 203 {
		t.Fatalf("unexpected latest sequence: %d, %v", latest, err)
	}
	batch, err := store.TaskEvents().Replay(ctx, tenantID, data.task.ID, 0, 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch) != 200 {
		t.Fatalf("unexpected capped replay length: %d", len(batch))
	}
	if batch[0].Sequence != 1 || batch[len(batch)-1].Sequence != 200 {
		t.Fatalf("unexpected capped replay boundaries: first=%#v last=%#v", batch[0], batch[len(batch)-1])
	}
	replay, err := store.TaskEvents().Replay(ctx, tenantID, data.task.ID, 200, 200)
	if err != nil || len(replay) != 3 || replay[0].Sequence != 201 || replay[2].Sequence != 203 {
		t.Fatalf("unexpected replay: %#v, %v", replay, err)
	}
	_, err = store.TaskEvents().Replay(ctx, "org:other", data.task.ID, 0, 1)
	requireCode(t, err, common.ResourceNotFound)
}

type taskFixture struct {
	now       time.Time
	timeRange common.AbsoluteTimeRange
	session   session.AnalysisSession
	message   session.Message
	task      task.AnalysisTask
}

func createTaskFixture(t *testing.T, ctx context.Context, store repositories.ApplicationStore) taskFixture {
	t.Helper()
	now := fixedNow()
	timeRange, err := common.NewAbsoluteTimeRange(now.Add(-30*time.Minute), now)
	if err != nil {
		t.Fatal(err)
	}
	analysisSession, err := session.New("session_1", tenantID, "Node exporter analysis", "user:1", now)
	if err != nil {
		t.Fatal(err)
	}
	message, err := session.NewMessage("message_1", tenantID, analysisSession.ID, session.RoleUser, "show node exporter", now)
	if err != nil {
		t.Fatal(err)
	}
	analysisTask, err := task.New("task_1", tenantID, analysisSession.ID, message.ID, "mock-prometheus", timeRange, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Sessions().Create(ctx, analysisSession); err != nil {
			return err
		}
		if err := tx.Messages().Append(ctx, message); err != nil {
			return err
		}
		return tx.Tasks().Create(ctx, analysisTask)
	}); err != nil {
		t.Fatal(err)
	}
	return taskFixture{now: now, timeRange: timeRange, session: analysisSession, message: message, task: analysisTask}
}

func openStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "ai-core.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return store
}

func fixedNow() time.Time { return time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC) }

func requireCode(t *testing.T, err error, want common.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error, got nil", want)
	}
	var domainErr *common.DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != want {
		t.Fatalf("expected %s error, got %v", want, err)
	}
}
