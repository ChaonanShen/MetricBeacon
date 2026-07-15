package incidents

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	storage "mini-torchbearing.local/services/ai-core/internal/adapters/outbound/storage/sqlite"
	"mini-torchbearing.local/services/ai-core/internal/application/workflows"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	domainincident "mini-torchbearing.local/services/ai-core/internal/domain/incident"
	"mini-torchbearing.local/services/ai-core/internal/domain/session"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
	"mini-torchbearing.local/services/ai-core/internal/ports/events"
	incidentport "mini-torchbearing.local/services/ai-core/internal/ports/incident"
)

func TestIngestCreatesOrgIncidentBeforeNotificationAndDiagnosesAsynchronously(t *testing.T) {
	ctx := context.Background()
	store, now := openServiceStore(t)
	toolset := &serviceToolset{}
	ids := &atomicIDs{}
	notifier := &durabilityCheckingNotifier{store: store}
	workflow := workflows.RunIncidentWorkflow{Store: store, Notifier: notifier, Toolset: toolset, IDs: ids, Clock: serviceClock{now}}
	service := New(serviceConfig(), store, notifier, toolset, workflow, ids, serviceClock{now})

	result, err := service.Ingest(ctx, firingAlert(now))
	if err != nil || !result.Accepted || result.Duplicate || result.TaskID == "" {
		t.Fatalf("ingest result: %#v, %v", result, err)
	}
	item := waitForDiagnosis(t, store, result.TaskID)
	if item.Kind != task.KindIncidentRemediation || item.IncidentPlan == nil || item.IncidentPlan.Diagnosis == nil || item.IncidentPlan.Intent != nil || item.Status != task.StatusRunningTools {
		t.Fatalf("incident task: %#v", item)
	}
	incidentSession, err := store.Sessions().Get(ctx, "org:1", item.SessionID)
	if err != nil || incidentSession.Kind != session.KindOrgIncident || incidentSession.OrgID != "1" || incidentSession.CreatedBy != "system:grafana" {
		t.Fatalf("incident session: %#v, %v", incidentSession, err)
	}
	if _, err := store.Sessions().GetOwned(ctx, "org:1", "user:1", item.SessionID); !hasCode(err, common.ResourceNotFound) {
		t.Fatalf("org incident leaked through private ownership lookup: %v", err)
	}
	messages, err := store.Messages().ListBySession(ctx, "org:1", item.SessionID)
	if err != nil || len(messages) != 1 || messages[0].Role != session.RoleTrigger {
		t.Fatalf("trigger messages: %#v, %v", messages, err)
	}
	if err := notifier.Err(); err != nil {
		t.Fatal(err)
	}
	if notifier.Count() < 15 {
		t.Fatalf("notification count = %d", notifier.Count())
	}

	duplicate, err := service.Ingest(ctx, firingAlert(now))
	if err != nil || !duplicate.Duplicate || duplicate.Accepted || duplicate.TaskID != result.TaskID || toolset.ResolveCalls() != 1 {
		t.Fatalf("duplicate result: %#v, calls=%d, err=%v", duplicate, toolset.ResolveCalls(), err)
	}
	key := alertKey(now, domainincident.AlertFiring)
	persisted, err := store.AlertEvents().GetByKey(ctx, key)
	if err != nil || persisted.TaskID != result.TaskID || persisted.Labels["service_ref"] != "order-demo" {
		t.Fatalf("alert event: %#v, %v", persisted, err)
	}
}

func TestIngestResolvedPersistsLifecycleWithoutCreatingAnotherTask(t *testing.T) {
	ctx := context.Background()
	store, now := openServiceStore(t)
	toolset := &serviceToolset{}
	ids := &atomicIDs{}
	workflow := workflows.RunIncidentWorkflow{Store: store, Toolset: toolset, IDs: ids, Clock: serviceClock{now}}
	service := New(serviceConfig(), store, nil, toolset, workflow, ids, serviceClock{now})
	firing, err := service.Ingest(ctx, firingAlert(now))
	if err != nil {
		t.Fatal(err)
	}
	waitForDiagnosis(t, store, firing.TaskID)
	resolvedAlert := firingAlert(now)
	resolvedAlert.Status = domainincident.AlertResolved
	resolved, err := service.Ingest(ctx, resolvedAlert)
	if err != nil || !resolved.Accepted || resolved.TaskID != firing.TaskID {
		t.Fatalf("resolved: %#v, %v", resolved, err)
	}
	retry, err := service.Ingest(ctx, resolvedAlert)
	if err != nil || !retry.Duplicate || retry.TaskID != firing.TaskID {
		t.Fatalf("resolved retry: %#v, %v", retry, err)
	}
	persisted, err := store.AlertEvents().GetByKey(ctx, alertKey(now, domainincident.AlertResolved))
	if err != nil || persisted.TaskID != firing.TaskID {
		t.Fatalf("resolved event: %#v, %v", persisted, err)
	}
	items, err := store.Tasks().ListNonTerminal(ctx)
	if err != nil || len(items) != 1 || items[0].ID != firing.TaskID {
		t.Fatalf("unexpected task fanout: %#v, %v", items, err)
	}
}

func TestConcurrentAlertRetriesCreateExactlyOneIncident(t *testing.T) {
	ctx := context.Background()
	store, now := openServiceStore(t)
	toolset := &serviceToolset{}
	ids := &atomicIDs{}
	workflow := workflows.RunIncidentWorkflow{Store: store, Toolset: toolset, IDs: ids, Clock: serviceClock{now}}
	service := New(serviceConfig(), store, nil, toolset, workflow, ids, serviceClock{now})

	const callers = 12
	results := make(chan Result, callers)
	errors := make(chan error, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := service.Ingest(ctx, firingAlert(now))
			results <- result
			errors <- err
		}()
	}
	group.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	accepted, duplicate := 0, 0
	var taskID string
	for result := range results {
		if result.Accepted {
			accepted++
		}
		if result.Duplicate {
			duplicate++
		}
		if taskID == "" {
			taskID = result.TaskID
		} else if taskID != result.TaskID {
			t.Fatalf("different tasks returned: %q and %q", taskID, result.TaskID)
		}
	}
	if accepted != 1 || duplicate != callers-1 {
		t.Fatalf("accepted=%d duplicate=%d", accepted, duplicate)
	}
	waitForDiagnosis(t, store, taskID)
	items, err := store.Tasks().ListNonTerminal(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("tasks: %#v, %v", items, err)
	}
}

func TestAlertMappingMismatchLeavesNoPersistentIncidentState(t *testing.T) {
	ctx := context.Background()
	store, now := openServiceStore(t)
	toolset := &serviceToolset{resolvedService: "another-service"}
	ids := &atomicIDs{}
	workflow := workflows.RunIncidentWorkflow{Store: store, Toolset: toolset, IDs: ids, Clock: serviceClock{now}}
	service := New(serviceConfig(), store, nil, toolset, workflow, ids, serviceClock{now})
	if _, err := service.Ingest(ctx, firingAlert(now)); !hasCode(err, common.SchemaValidationFailed) {
		t.Fatalf("error = %v", err)
	}
	if _, err := store.AlertEvents().GetByKey(ctx, alertKey(now, domainincident.AlertFiring)); !hasCode(err, common.ResourceNotFound) {
		t.Fatalf("unexpected alert state: %v", err)
	}
	items, err := store.Tasks().ListNonTerminal(ctx)
	if err != nil || len(items) != 0 {
		t.Fatalf("unexpected task state: %#v, %v", items, err)
	}
}

type serviceToolset struct {
	resolveCalls    atomic.Int64
	resolvedService string
}

func (f *serviceToolset) ResolveAndStart(context.Context, requestcontext.Context, string, string, map[string]string) (incidentport.ResolvedRun, error) {
	f.resolveCalls.Add(1)
	serviceRef := f.resolvedService
	if serviceRef == "" {
		serviceRef = "order-demo"
	}
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	return incidentport.ResolvedRun{MappingID: "mapping.order-queue", MappingDigest: digest, PlaybookID: "order.queue-backlog", PlaybookVersion: "1.0.0", PlaybookDigest: digest, ServiceRef: serviceRef, Checkpoint: "opaque-signed-checkpoint", AssetRefs: []task.AssetRef{{Kind: "alert_mapping", ID: "mapping.order-queue", Version: "1", Digest: digest}, {Kind: "knowledge", ID: "order-service", Version: "1.0.0", Digest: digest}, {Kind: "skill", ID: "diagnose-worker", Version: "1.0.0", Digest: digest}, {Kind: "playbook", ID: "order.queue-backlog", Version: "1.0.0", Digest: digest}}}, nil
}

func (f *serviceToolset) Observe(context.Context, requestcontext.Context, string) (incidentport.Observation, error) {
	names := []string{"order_service.get_queue_snapshot", "order_service.get_worker_state", "order_service.get_worker_policy", "order_service.get_recent_outcomes"}
	evidence := make([]incidentport.ToolEvidence, 0, len(names))
	for _, name := range names {
		evidence = append(evidence, incidentport.ToolEvidence{Name: name, InputSummary: json.RawMessage(`{}`), OutputSummary: json.RawMessage(`{"bounded":true}`), DurationMS: 1})
	}
	return incidentport.Observation{Diagnosis: task.Diagnosis{PrimaryHypothesis: "worker_stopped", EvidenceRefs: names, AlternativeHypotheses: []string{"slow_processing", "dependency_errors"}, Confidence: 0.99, CandidateAction: "restore_worker_concurrency"}, Evidence: evidence}, nil
}

func (f *serviceToolset) Prepare(context.Context, requestcontext.Context, string, task.Diagnosis) (incidentport.PreparedRun, error) {
	digest := strings.Repeat("a", 64)
	return incidentport.PreparedRun{Status: "needs_approval", Checkpoint: "prepared-checkpoint", Intent: &incidentport.PreparedIntent{CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: "order-demo", InstanceEpoch: "epoch-1", ExpectedVersion: 2, ObservedAt: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC), PolicyDigest: digest, PlaybookDigest: digest, BeforeConcurrency: 0, AfterConcurrency: 2, RiskSummary: "bounded restore"}}, nil
}

func (f *serviceToolset) ResolveCalls() int64 { return f.resolveCalls.Load() }

type atomicIDs struct{ next atomic.Int64 }

func (g *atomicIDs) NewID(kind string) string {
	return fmt.Sprintf("%s_%04d", kind, g.next.Add(1))
}

type serviceClock struct{ value time.Time }

func (c serviceClock) Now() time.Time { return c.value }

type durabilityCheckingNotifier struct {
	store *storage.Store
	mu    sync.Mutex
	count int
	err   error
}

var _ events.Notifier = (*durabilityCheckingNotifier)(nil)

func (n *durabilityCheckingNotifier) Notify(ctx context.Context, event task.TaskEvent) error {
	replayed, err := n.store.TaskEvents().ReplayTo(ctx, event.TenantID, event.TaskID, event.Sequence-1, event.Sequence, 1)
	n.mu.Lock()
	defer n.mu.Unlock()
	n.count++
	if err != nil {
		n.err = err
	} else if len(replayed) != 1 || replayed[0].EventID != event.EventID {
		n.err = fmt.Errorf("event %s was notified before durable persistence", event.EventID)
	}
	return nil
}

func (n *durabilityCheckingNotifier) Subscribe(context.Context, string, string) (<-chan struct{}, error) {
	return make(chan struct{}), nil
}

func (n *durabilityCheckingNotifier) Count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.count
}

func (n *durabilityCheckingNotifier) Err() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.err
}

func openServiceStore(t *testing.T) (*storage.Store, time.Time) {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "ai-core.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, time.Date(2026, 7, 16, 11, 0, 0, 0, time.UTC)
}

func serviceConfig() Config {
	return Config{TenantID: "org:1", OrgID: "1", ActorID: "system:grafana"}
}

func firingAlert(now time.Time) Alert {
	return Alert{SourceID: "demo-grafana", AlertName: "OrderQueueBacklog", Fingerprint: "fingerprint-1", ServiceRef: "order-demo", Status: domainincident.AlertFiring, Labels: map[string]string{"service_ref": "order-demo", "severity": "warning"}, StartsAt: now.Add(-time.Minute), RequestID: "request-1", TraceID: "trace-1"}
}

func alertKey(now time.Time, status domainincident.AlertStatus) domainincident.AlertKey {
	return domainincident.AlertKey{TenantID: "org:1", OrgID: "1", SourceID: "demo-grafana", Fingerprint: "fingerprint-1", StartsAt: now.Add(-time.Minute), Status: status}
}

func waitForDiagnosis(t *testing.T, store *storage.Store, taskID string) task.AnalysisTask {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		item, err := store.Tasks().Get(context.Background(), "org:1", taskID)
		if err == nil && item.IncidentPlan != nil && item.IncidentPlan.Diagnosis != nil {
			return item
		}
		time.Sleep(5 * time.Millisecond)
	}
	item, err := store.Tasks().Get(context.Background(), "org:1", taskID)
	t.Fatalf("incident diagnosis timed out: %#v, %v", item, err)
	return task.AnalysisTask{}
}
