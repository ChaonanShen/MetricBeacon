package prometheusadapter

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"mini-torchbearing.local/services/order-demo/internal/application"
)

func TestMetricsReflectEngineStateAndRealBusinessEvents(t *testing.T) {
	recorder := NewRecorder()
	var id atomic.Int64
	engine := application.NewEngine(application.Options{ProcessingDelay: 5 * time.Millisecond, Metrics: recorder,
		ID: func() string { return fmt.Sprintf("id-%d", id.Add(1)) }})
	t.Cleanup(engine.Close)
	recorder.BindEngine(engine)
	if _, err := engine.Submit("key-1", "DEMO", 1); err != nil {
		t.Fatal(err)
	}
	waitMetric(t, func() bool { return testutil.ToFloat64(recorder.completed.WithLabelValues("completed")) == 1 })
	if got := testutil.ToFloat64(recorder.received.WithLabelValues("accepted")); got != 1 {
		t.Fatalf("received = %v", got)
	}
	if err := engine.ActivateFault(application.FaultWorkerStopped); err != nil {
		t.Fatal(err)
	}
	waitMetric(t, func() bool { return engine.WorkerSnapshot().ActiveWorkers == 0 })
	if _, err := engine.Submit("key-2", "DEMO", 1); err != nil {
		t.Fatal(err)
	}
	families, err := recorder.Gatherer().Gather()
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]float64{}
	for _, family := range families {
		if len(family.Metric) > 0 && family.Metric[0].Gauge != nil {
			values[family.GetName()] = family.Metric[0].Gauge.GetValue()
		}
	}
	if values["mtb_demo_order_queue_depth"] != 1 || values["mtb_demo_worker_configured"] != 0 || values["mtb_demo_worker_active"] != 0 {
		t.Fatalf("gauges = %#v", values)
	}
}

func waitMetric(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
