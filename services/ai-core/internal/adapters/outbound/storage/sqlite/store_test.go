package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	storage "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/storage/sqlite"
	"mini-torchbearing.local/services/ai-core/internal/domain/chart"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/incident"
	"mini-torchbearing.local/services/ai-core/internal/domain/remediation"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/repositories"
	migrations "mini-torchbearing.local/services/ai-core/migrations/sqlite"
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
	if !reflect.DeepEqual(loadedTask.QueryPlan, data.task.QueryPlan) {
		t.Fatalf("query plan = %#v, want %#v", loadedTask.QueryPlan, data.task.QueryPlan)
	}
	metricCheckpoint, _ := task.NewCheckpoint(data.task.ID, tenantID, task.PhaseNeedsAgent, "must-not-persist", data.now)
	requireCode(t, store.TaskCheckpoints().Create(ctx, metricCheckpoint), common.ResourceNotFound)
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
		SourceCallID: "source_1",
		ToolName:     "grafana.search_metrics",
		ToolVersion:  "v1",
		Status:       task.ToolCallStarted,
		InputSummary: []byte(`{"datasourceUid":"prometheus-main"}`),
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

	query := chart.QuerySpec{RefID: "A", Expression: "up", Legend: "{{instance}}", DatasourceUID: "prometheus-main", TimeRange: data.timeRange, StepSeconds: 300}
	draft, err := chart.New("chart_1", tenantID, data.session.ID, data.task.ID, "CPU usage", "percent", []chart.QuerySpec{query}, data.now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Charts().Create(ctx, draft); err != nil {
		t.Fatal(err)
	}
	execution := chart.Execution{
		ID:                "execution_1",
		TenantID:          tenantID,
		ChartID:           draft.ID,
		QueryRefID:        "A",
		Status:            chart.ExecutionSuccess,
		DurationMS:        12,
		SampleRange:       data.timeRange,
		ActualSampleRange: &common.TimeBounds{From: data.timeRange.From, To: data.timeRange.From},
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
	if err != nil || len(executions) != 1 || len(executions[0].Series) != 1 || executions[0].ActualSampleRange == nil || executions[0].ActualSampleRange.From != data.timeRange.From {
		t.Fatalf("unexpected executions: %#v, %v", executions, err)
	}

	_, err = store.Sessions().Get(ctx, "org:other", data.session.ID)
	requireCode(t, err, common.ResourceNotFound)
	_, err = store.Tasks().Get(ctx, "org:other", data.task.ID)
	requireCode(t, err, common.ResourceNotFound)
	forged, err := session.NewMessage("message_forged", "org:other", data.session.ID, "task_forged", session.RoleUser, "forged", data.now)
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
	rolledBack, err := session.NewPrivate("session_rollback", tenantID, "1", "rolled back", "user:1", now)
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

func TestSessionHistoryIsOwnerScopedNonEmptyAndKeysetPaged(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)
	now := fixedNow()
	createSessionHistoryFixture(t, ctx, store, tenantID, "user:1", "session_old", now.Add(-2*time.Minute), true)
	createSessionHistoryFixture(t, ctx, store, tenantID, "user:1", "session_same_a", now, true)
	createSessionHistoryFixture(t, ctx, store, tenantID, "user:1", "session_same_b", now, true)
	createSessionHistoryFixture(t, ctx, store, tenantID, "user:1", "session_empty", now.Add(time.Minute), false)
	createSessionHistoryFixture(t, ctx, store, tenantID, "user:2", "session_other_user", now.Add(2*time.Minute), true)
	createSessionHistoryFixture(t, ctx, store, "org:2", "user:1", "session_other_tenant", now.Add(3*time.Minute), true)

	first, err := store.Sessions().ListPageByOwner(ctx, tenantID, "user:1", repositories.SessionListRequest{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got := sessionIDs(first.Items); !reflect.DeepEqual(got, []string{"session_same_b", "session_same_a"}) || first.NextAfter == nil {
		t.Fatalf("first page = %#v cursor=%#v", got, first.NextAfter)
	}
	second, err := store.Sessions().ListPageByOwner(ctx, tenantID, "user:1", repositories.SessionListRequest{Limit: 2, BeforeUpdatedAt: &first.NextAfter.UpdatedAt, BeforeID: first.NextAfter.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := sessionIDs(second.Items); !reflect.DeepEqual(got, []string{"session_old"}) || second.NextAfter != nil {
		t.Fatalf("second page = %#v cursor=%#v", got, second.NextAfter)
	}
	owned, err := store.Sessions().GetOwned(ctx, tenantID, "user:1", "session_old")
	if err != nil || owned.CreatedBy != "user:1" {
		t.Fatalf("owned session = %#v, %v", owned, err)
	}
	_, err = store.Sessions().GetOwned(ctx, tenantID, "user:2", "session_old")
	requireCode(t, err, common.ResourceNotFound)
	_, err = store.Sessions().ListPageByOwner(ctx, tenantID, "user:1", repositories.SessionListRequest{Limit: 0})
	requireCode(t, err, common.InvalidArgument)
}

func TestSessionHistoryIndexMigratesExistingDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "v1-session-history.sqlite")
	seedV1Database(t, path, false)
	store, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var found int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_index_list('sessions') WHERE name = 'sessions_tenant_creator_updated_id_idx'`).Scan(&found); err != nil || found != 1 {
		t.Fatalf("session history index count = %d, %v", found, err)
	}
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

func TestMultiTurnMigrationBackfillsMessagesAndSourceCallIDs(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "v1.sqlite")
	seedV1Database(t, path, false)

	store, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	messages, err := store.Messages().ListBySession(ctx, tenantID, "session_1")
	if err != nil || len(messages) != 2 {
		t.Fatalf("messages: %#v, %v", messages, err)
	}
	for _, message := range messages {
		if message.TaskID != "task_1" {
			t.Fatalf("message task id = %q, want task_1", message.TaskID)
		}
	}
	calls, err := store.ToolCalls().ListByTask(ctx, tenantID, "task_1")
	if err != nil || len(calls) != 1 || calls[0].SourceCallID != "legacy:tool_1" {
		t.Fatalf("tool calls: %#v, %v", calls, err)
	}
	loadedTask, err := store.Tasks().Get(ctx, tenantID, "task_1")
	wantPlan, _ := task.NewQueryPlan([]string{"cpu"}, 300, 300)
	if err != nil || loadedTask.DatasourceUID != "prometheus-main" || !reflect.DeepEqual(loadedTask.QueryPlan, wantPlan) {
		t.Fatalf("task datasource: %#v, %v", loadedTask, err)
	}
	charts, err := store.Charts().ListByTask(ctx, tenantID, "task_1")
	if err != nil || len(charts) != 1 || charts[0].Queries[0].DatasourceUID != "prometheus-main" || charts[0].Queries[0].StepSeconds != 300 {
		t.Fatalf("chart datasource: %#v, %v", charts, err)
	}
}

func TestMultiTurnMigrationRejectsAmbiguousActiveTasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ambiguous-v1.sqlite")
	seedV1Database(t, path, true)
	_, err := storage.Open(context.Background(), path)
	requireCode(t, err, common.InvalidStateTransition)
}

func TestIncidentTaskAndCheckpointSurviveReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "incident.sqlite")
	store, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	now := fixedNow()
	incidentSession, err := session.NewIncident("session_incident", tenantID, "1", "Order backlog", "system:grafana", now)
	if err != nil {
		t.Fatal(err)
	}
	trigger, err := session.NewMessage("message_trigger", tenantID, incidentSession.ID, "task_incident", session.RoleTrigger, "OrderQueueBacklog firing", now)
	if err != nil {
		t.Fatal(err)
	}
	incidentTask, err := task.NewIncident("task_incident", tenantID, incidentSession.ID, trigger.ID, storedIncidentPlan(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Sessions().Create(ctx, incidentSession); err != nil {
			return err
		}
		if err := tx.Messages().Append(ctx, trigger); err != nil {
			return err
		}
		return tx.Tasks().Create(ctx, incidentTask)
	}); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := task.NewCheckpoint(incidentTask.ID, tenantID, task.PhaseNeedsAgent, "signed:checkpoint:one", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.TaskCheckpoints().Create(ctx, checkpoint); err != nil {
		t.Fatal(err)
	}
	alertKey := incident.AlertKey{TenantID: tenantID, OrgID: "1", SourceID: "demo-grafana", Fingerprint: "fingerprint_1", StartsAt: now, Status: incident.AlertFiring}
	alertEvent, err := incident.NewAlertEvent("alert_1", alertKey, "order-demo", "OrderQueueBacklog", map[string]string{"alertname": "OrderQueueBacklog", "service_ref": "order-demo"}, incidentTask.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AlertEvents().Create(ctx, alertEvent); err != nil {
		t.Fatal(err)
	}
	requireCode(t, store.AlertEvents().Create(ctx, alertEvent), common.ResourceConflict)
	if _, err := store.TaskEvents().Append(ctx, task.EventDraft{EventID: "event_alert", TenantID: tenantID, TaskID: incidentTask.ID, SessionID: incidentSession.ID, Type: task.EventAlertReceived, Timestamp: now, Payload: []byte(`{"sourceId":"demo-grafana","alertName":"OrderQueueBacklog","fingerprint":"fingerprint_1","serviceRef":"order-service:demo","status":"firing","startsAt":"2026-07-13T10:30:00Z"}`)}); err != nil {
		t.Fatal(err)
	}
	stale := checkpoint
	if err := stale.Replace(task.PhasePrepare, "signed:checkpoint:two", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.TaskCheckpoints().Update(ctx, stale, checkpoint.Version+1); err == nil {
		t.Fatal("checkpoint update with wrong expected version unexpectedly succeeded")
	}
	if err := store.TaskCheckpoints().Update(ctx, stale, checkpoint.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Sessions().GetOwned(ctx, tenantID, "system:grafana", incidentSession.ID); err == nil {
		t.Fatal("org Incident leaked through private owner lookup")
	}
	duplicateSession, _ := session.NewIncident("session_duplicate", tenantID, "1", "Duplicate", "system:grafana", now)
	duplicateTrigger, _ := session.NewMessage("message_duplicate", tenantID, duplicateSession.ID, "task_duplicate", session.RoleTrigger, "same alert", now)
	duplicateTask, _ := task.NewIncident("task_duplicate", tenantID, duplicateSession.ID, duplicateTrigger.ID, storedIncidentPlan(), now)
	err = store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Sessions().Create(ctx, duplicateSession); err != nil {
			return err
		}
		if err := tx.Messages().Append(ctx, duplicateTrigger); err != nil {
			return err
		}
		return tx.Tasks().Create(ctx, duplicateTask)
	})
	requireCode(t, err, common.ResourceConflict)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.Tasks().Get(ctx, tenantID, incidentTask.ID)
	if err != nil || loaded.Kind != task.KindIncidentRemediation || loaded.IncidentPlan == nil || loaded.IncidentPlan.AlertFingerprint != "fingerprint_1" || loaded.QueryPlan.Valid() || loaded.DatasourceUID != "" || loaded.LatestSequence != 1 {
		t.Fatalf("reopened Incident Task = %#v, %v", loaded, err)
	}
	loadedCheckpoint, err := reopened.TaskCheckpoints().Get(ctx, tenantID, incidentTask.ID)
	if err != nil || loadedCheckpoint.Version != 2 || loadedCheckpoint.OpaqueValue != "signed:checkpoint:two" {
		t.Fatalf("reopened checkpoint = %#v, %v", loadedCheckpoint, err)
	}
	loadedAlert, err := reopened.AlertEvents().GetByKey(ctx, alertKey)
	if err != nil || loadedAlert.TaskID != incidentTask.ID || !reflect.DeepEqual(loadedAlert.Labels, alertEvent.Labels) {
		t.Fatalf("reopened AlertEvent = %#v, %v", loadedAlert, err)
	}
	otherTenantKey := alertKey
	otherTenantKey.TenantID = "org:other"
	if _, err := reopened.AlertEvents().GetByKey(ctx, otherTenantKey); err == nil {
		t.Fatal("AlertEvent crossed tenant boundary")
	}
	if _, err := reopened.TaskCheckpoints().Get(ctx, "org:other", incidentTask.ID); err == nil {
		t.Fatal("checkpoint crossed tenant boundary")
	}
	events, err := reopened.TaskEvents().Replay(ctx, tenantID, incidentTask.ID, 0, 10)
	if err != nil || len(events) != 1 || events[0].Type != task.EventAlertReceived {
		t.Fatalf("reopened Incident events = %#v, %v", events, err)
	}
	rows, err := reopened.Tasks().ListNonTerminal(ctx)
	if err != nil || len(rows) != 1 || rows[0].ID != incidentTask.ID {
		t.Fatalf("nonterminal Incidents = %#v, %v", rows, err)
	}
	requireCode(t, reopened.TaskCheckpoints().Delete(ctx, tenantID, incidentTask.ID, loadedCheckpoint.Version-1), common.ResourceConflict)
	if err := reopened.TaskCheckpoints().Delete(ctx, tenantID, incidentTask.ID, loadedCheckpoint.Version); err != nil {
		t.Fatal(err)
	}
	_, err = reopened.TaskCheckpoints().Get(ctx, tenantID, incidentTask.ID)
	requireCode(t, err, common.ResourceNotFound)
}

func TestRemediationLifecyclePersistsCASAndTenantScopeAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "remediation.sqlite")
	store, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	now := fixedNow()
	incidentTask := createIncidentFixture(t, ctx, store, "lifecycle", now)
	intent := remediation.Intent{ID: "intent_lifecycle", Digest: "sha256:" + strings.Repeat("e", 64), CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: incidentTask.IncidentPlan.ServiceRef, InstanceEpoch: "epoch_lifecycle", ExpectedVersion: 2, BeforeConcurrency: 0, AfterConcurrency: 2, Risk: "low", CreatedAt: now}
	wrongScope, _ := remediation.NewIntentRecord(tenantID, "other-org", incidentTask.ID, intent)
	requireCode(t, store.RemediationIntents().Create(ctx, wrongScope), common.ResourceConflict)
	intentRecord, err := remediation.NewIntentRecord(tenantID, "1", incidentTask.ID, intent)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := remediation.NewApproval("approval_lifecycle", tenantID, "1", incidentTask.ID, intent.ID, intent.Digest, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.RemediationIntents().Create(ctx, intentRecord); err != nil {
			return err
		}
		return tx.Approvals().Create(ctx, approval)
	}); err != nil {
		t.Fatal(err)
	}
	prematureExecution, _ := remediation.NewExecution("operation_premature", tenantID, "1", incidentTask.ID, approval.ID, intent.Digest, intent.InstanceEpoch, intent.ExpectedVersion, now.Add(30*time.Second))
	requireCode(t, store.RemediationExecutions().Create(ctx, prematureExecution), common.ResourceConflict)

	first, second := approval, approval
	if err := first.Decide(remediation.DecisionApprove, "admin:1", "approved once", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := second.Decide(remediation.DecisionApprove, "admin:2", "concurrent retry", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for _, candidate := range []remediation.Approval{first, second} {
		candidate := candidate
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			errs <- store.Approvals().Update(ctx, candidate, approval.Version)
		}()
	}
	close(start)
	wait.Wait()
	close(errs)
	succeeded, conflicts := 0, 0
	for updateErr := range errs {
		if updateErr == nil {
			succeeded++
			continue
		}
		var domainErr *common.DomainError
		if errors.As(updateErr, &domainErr) && domainErr.Code == common.ResourceConflict {
			conflicts++
			continue
		}
		t.Fatalf("approval CAS error: %v", updateErr)
	}
	if succeeded != 1 || conflicts != 1 {
		t.Fatalf("approval CAS succeeded=%d conflicts=%d", succeeded, conflicts)
	}

	execution, err := remediation.NewExecution("operation_lifecycle", tenantID, "1", incidentTask.ID, approval.ID, intent.Digest, intent.InstanceEpoch, intent.ExpectedVersion, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RemediationExecutions().Create(ctx, execution); err != nil {
		t.Fatal(err)
	}
	if err := execution.MarkUnknown(now.Add(2*time.Minute + time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.RemediationExecutions().Update(ctx, execution, 1); err != nil {
		t.Fatal(err)
	}
	audit, err := remediation.NewAuditRecord("audit_lifecycle", tenantID, "1", incidentTask.ID, "admin:1", remediation.AuditApprovalDecision, remediation.AuditAccepted, "approval accepted", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AuditRecords().Create(ctx, audit); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loadedIntent, err := reopened.RemediationIntents().GetByTask(ctx, tenantID, incidentTask.ID)
	if err != nil || loadedIntent.Intent.Digest != intent.Digest {
		t.Fatalf("intent=%#v err=%v", loadedIntent, err)
	}
	loadedApproval, err := reopened.Approvals().GetByTask(ctx, tenantID, incidentTask.ID)
	if err != nil || loadedApproval.Status != remediation.ApprovalApproved || loadedApproval.Version != 2 {
		t.Fatalf("approval=%#v err=%v", loadedApproval, err)
	}
	loadedExecution, err := reopened.RemediationExecutions().GetByOperation(ctx, tenantID, execution.OperationID)
	if err != nil || loadedExecution.State != remediation.ExecutionUnknown || loadedExecution.Version != 2 {
		t.Fatalf("execution=%#v err=%v", loadedExecution, err)
	}
	if err := loadedExecution.RecordReceipt(2, 3, true, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := reopened.RemediationExecutions().Update(ctx, loadedExecution, 2); err != nil {
		t.Fatal(err)
	}
	loadedByTask, err := reopened.RemediationExecutions().GetByTask(ctx, tenantID, incidentTask.ID)
	if err != nil || loadedByTask.State != remediation.ExecutionAlreadyApplied || loadedByTask.Version != 3 {
		t.Fatalf("reconciled=%#v err=%v", loadedByTask, err)
	}
	audits, err := reopened.AuditRecords().ListByTask(ctx, tenantID, incidentTask.ID)
	if err != nil || len(audits) != 1 || audits[0].Action != remediation.AuditApprovalDecision {
		t.Fatalf("audits=%#v err=%v", audits, err)
	}
	if _, err := reopened.Approvals().GetByTask(ctx, "org:other", incidentTask.ID); err == nil {
		t.Fatal("approval crossed tenant boundary")
	}
	if _, err := reopened.RemediationExecutions().GetByOperation(ctx, "org:other", execution.OperationID); err == nil {
		t.Fatal("execution crossed tenant boundary")
	}
}

func TestIncidentMigrationPreservesLegacyGraphAndForeignKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-v1.sqlite")
	seedV1Database(t, path, false)
	store, err := storage.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	legacyTask, err := store.Tasks().Get(context.Background(), tenantID, "task_1")
	if err != nil || legacyTask.Kind != task.KindMetricAnalysis || legacyTask.IncidentPlan != nil {
		t.Fatalf("legacy Task = %#v, %v", legacyTask, err)
	}
	legacySession, err := store.Sessions().Get(context.Background(), tenantID, "session_1")
	if err != nil || legacySession.Kind != session.KindPrivate || legacySession.OrgID != "1" {
		t.Fatalf("legacy Session = %#v, %v", legacySession, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var table string
	if err := db.QueryRow(`SELECT "table" FROM pragma_foreign_key_check LIMIT 1`).Scan(&table); err != sql.ErrNoRows {
		t.Fatalf("foreign_key_check = %q, %v", table, err)
	}
}

func TestIncidentMigrationUpgradesVersionSixDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-v6.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{migrations.Initial, migrations.MultiTurnAndToolCorrelation, migrations.DatasourceUID, migrations.BoundedQueryPlan, migrations.QueryPlanViews, migrations.SessionHistoryIndex} {
		if _, err := db.Exec(source); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL); INSERT INTO schema_migrations VALUES (1, '2026-07-16T00:00:00.000000000Z'), (2, '2026-07-16T00:00:00.000000000Z'), (3, '2026-07-16T00:00:00.000000000Z'), (4, '2026-07-16T00:00:00.000000000Z'), (5, '2026-07-16T00:00:00.000000000Z'), (6, '2026-07-16T00:00:00.000000000Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`SELECT version FROM schema_migrations WHERE version = 8`).Scan(&version); err != nil || version != 8 {
		t.Fatalf("migration version = %d, %v", version, err)
	}
	var kindColumn int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name = 'kind' AND [notnull] = 1`).Scan(&kindColumn); err != nil || kindColumn != 1 {
		t.Fatalf("Task kind column = %d, %v", kindColumn, err)
	}
}

func TestOnlyOneConcurrentActiveTaskCanBeCreatedForSession(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)
	now := fixedNow()
	analysisSession, err := session.NewPrivate("session_concurrent", tenantID, "1", "Concurrent", "user:1", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Sessions().Create(ctx, analysisSession); err != nil {
		t.Fatal(err)
	}
	timeRange, _ := common.NewAbsoluteTimeRange(now.Add(-time.Minute), now)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 1; index <= 2; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			taskID := fmt.Sprintf("task_concurrent_%d", index)
			message, _ := session.NewMessage(fmt.Sprintf("message_concurrent_%d", index), tenantID, analysisSession.ID, taskID, session.RoleUser, "show cpu", now)
			item, _ := task.New(taskID, tenantID, analysisSession.ID, message.ID, "prometheus-main", timeRange, task.LegacyQueryPlan(), now)
			errs <- store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
				if err := tx.Messages().Append(ctx, message); err != nil {
					return err
				}
				return tx.Tasks().Create(ctx, item)
			})
		}()
	}
	close(start)
	wait.Wait()
	close(errs)
	succeeded, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			succeeded++
			continue
		}
		var domainErr *common.DomainError
		if errors.As(err, &domainErr) && domainErr.Code == common.ResourceConflict {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent create error: %v", err)
	}
	if succeeded != 1 || conflicts != 1 {
		t.Fatalf("succeeded=%d conflicts=%d, want 1/1", succeeded, conflicts)
	}
}

func seedV1Database(t *testing.T, path string, duplicateActive bool) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(migrations.Initial); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL); INSERT INTO schema_migrations (version, applied_at) VALUES (1, '2026-07-13T10:30:00.000000000Z');`); err != nil {
		t.Fatal(err)
	}
	stamp := "2026-07-13T10:30:00.000000000Z"
	if _, err := db.Exec(`INSERT INTO sessions (id, tenant_id, title, status, created_by, created_at, updated_at, version) VALUES ('session_1', ?, 'Overview', 'active', 'user:1', ?, ?, 1)`, tenantID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO messages (id, tenant_id, session_id, role, content, created_at) VALUES ('message_user', ?, 'session_1', 'user', 'show node exporter', ?)`, tenantID, stamp); err != nil {
		t.Fatal(err)
	}
	status := "completed"
	if duplicateActive {
		status = "created"
	}
	if _, err := db.Exec(`INSERT INTO tasks (id, tenant_id, session_id, status, input_message_id, datasource_uid, time_from, time_to, latest_sequence, created_at, updated_at, version) VALUES ('task_1', ?, 'session_1', ?, 'message_user', 'mock-prometheus', ?, '2026-07-13T10:30:00.000000000Z', 1, ?, ?, 1)`, tenantID, status, "2026-07-13T10:00:00.000000000Z", stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if duplicateActive {
		if _, err := db.Exec(`INSERT INTO messages (id, tenant_id, session_id, role, content, created_at) VALUES ('message_user_2', ?, 'session_1', 'user', 'second request', ?)`, tenantID, stamp); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO tasks (id, tenant_id, session_id, status, input_message_id, datasource_uid, time_from, time_to, latest_sequence, created_at, updated_at, version) VALUES ('task_2', ?, 'session_1', 'created', 'message_user_2', 'mock-prometheus', ?, '2026-07-13T10:30:00.000000000Z', 0, ?, ?, 1)`, tenantID, "2026-07-13T10:00:00.000000000Z", stamp, stamp); err != nil {
			t.Fatal(err)
		}
		return
	}
	if _, err := db.Exec(`INSERT INTO charts (id, tenant_id, session_id, task_id, title, visualization, unit, queries_json, status, created_at, updated_at, version) VALUES ('chart_1', ?, 'session_1', 'task_1', 'CPU', 'timeseries', 'percent', '[{"refId":"A","expression":"100 * (1 - avg(rate(node_cpu_seconds_total{mode=\"idle\"}[5m])))","legend":"{{instance}}","datasourceUid":"mock-prometheus","timeRange":{"from":"2026-07-13T10:00:00Z","to":"2026-07-13T10:30:00Z"}}]', 'ready', ?, ?, 1)`, tenantID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO messages (id, tenant_id, session_id, role, content, created_at) VALUES ('message_assistant', ?, 'session_1', 'assistant', 'result', ?)`, tenantID, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO task_events (event_id, tenant_id, task_id, session_id, sequence, type, timestamp, payload_json) VALUES ('event_1', ?, 'task_1', 'session_1', 1, 'assistant.message.completed', ?, '{"message":{"id":"message_assistant"}}')`, tenantID, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO tool_calls (id, tenant_id, task_id, tool_name, tool_version, status, input_summary_json, started_at, version) VALUES ('tool_1', ?, 'task_1', 'grafana.search_metrics', 'v1', 'started', '{}', ?, 1)`, tenantID, stamp); err != nil {
		t.Fatal(err)
	}
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
	analysisSession, err := session.NewPrivate("session_1", tenantID, "1", "Node exporter analysis", "user:1", now)
	if err != nil {
		t.Fatal(err)
	}
	message, err := session.NewMessage("message_1", tenantID, analysisSession.ID, "task_1", session.RoleUser, "show node exporter", now)
	if err != nil {
		t.Fatal(err)
	}
	queryPlan, err := task.NewQueryPlan([]string{"memory", "cpu"}, 10, 60)
	if err != nil {
		t.Fatal(err)
	}
	analysisTask, err := task.New("task_1", tenantID, analysisSession.ID, message.ID, "prometheus-main", timeRange, queryPlan, now)
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

func createSessionHistoryFixture(t *testing.T, ctx context.Context, store repositories.ApplicationStore, tenant, owner, id string, updatedAt time.Time, withTask bool) {
	t.Helper()
	createdAt := fixedNow().Add(-time.Hour)
	value, err := session.NewPrivate(id, tenant, "1", id, owner, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := value.Touch(updatedAt); err != nil {
		t.Fatal(err)
	}
	if !withTask {
		if err := store.Sessions().Create(ctx, value); err != nil {
			t.Fatal(err)
		}
		return
	}
	taskID := "task_" + id
	message, err := session.NewMessage("message_"+id, tenant, id, taskID, session.RoleUser, id, updatedAt)
	if err != nil {
		t.Fatal(err)
	}
	timeRange, err := common.NewAbsoluteTimeRange(updatedAt.Add(-time.Minute), updatedAt)
	if err != nil {
		t.Fatal(err)
	}
	analysisTask, err := task.New(taskID, tenant, id, message.ID, "prometheus-main", timeRange, task.LegacyQueryPlan(), updatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Sessions().Create(ctx, value); err != nil {
			return err
		}
		if err := tx.Messages().Append(ctx, message); err != nil {
			return err
		}
		return tx.Tasks().Create(ctx, analysisTask)
	}); err != nil {
		t.Fatal(err)
	}
}

func sessionIDs(items []session.AnalysisSession) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
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

func createIncidentFixture(t *testing.T, ctx context.Context, store repositories.ApplicationStore, suffix string, now time.Time) task.AnalysisTask {
	t.Helper()
	sessionID, taskID, messageID := "session_"+suffix, "task_"+suffix, "message_"+suffix
	incidentSession, err := session.NewIncident(sessionID, tenantID, "1", "Order backlog", "system:grafana", now)
	if err != nil {
		t.Fatal(err)
	}
	trigger, err := session.NewMessage(messageID, tenantID, sessionID, taskID, session.RoleTrigger, "OrderQueueBacklog firing", now)
	if err != nil {
		t.Fatal(err)
	}
	incidentTask, err := task.NewIncident(taskID, tenantID, sessionID, messageID, storedIncidentPlan(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WithinTransaction(ctx, func(tx repositories.ApplicationStore) error {
		if err := tx.Sessions().Create(ctx, incidentSession); err != nil {
			return err
		}
		if err := tx.Messages().Append(ctx, trigger); err != nil {
			return err
		}
		return tx.Tasks().Create(ctx, incidentTask)
	}); err != nil {
		t.Fatal(err)
	}
	return incidentTask
}

func storedIncidentPlan() task.IncidentPlan {
	digest := func(char string) string { return "sha256:" + strings.Repeat(char, 64) }
	return task.IncidentPlan{
		SourceID: "demo-grafana", AlertName: "OrderQueueBacklog", AlertFingerprint: "fingerprint_1", ServiceRef: "order-service:demo",
		Labels:  map[string]string{"alertname": "OrderQueueBacklog", "service_ref": "order-service:demo"},
		Mapping: task.PinnedRef{ID: "order-demo", Digest: digest("a")}, Playbook: task.PinnedRef{ID: "order-queue-backlog", Version: "1", Digest: digest("b")},
		AssetRefs: []task.AssetRef{{Kind: "alert_mapping", ID: "order-demo", Version: "1", Digest: digest("a")}, {Kind: "playbook", ID: "order-queue-backlog", Version: "1", Digest: digest("b")}, {Kind: "knowledge", ID: "order-service", Version: "1", Digest: digest("c")}, {Kind: "skill", ID: "diagnose-order-backlog", Version: "1", Digest: digest("d")}},
		Phase:     task.PhaseNeedsAgent,
	}
}

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
