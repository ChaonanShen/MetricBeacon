package prometheusadapter

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"mini-torchbearing.local/services/order-demo/internal/application"
)

type Recorder struct {
	registry   *prometheus.Registry
	received   *prometheus.CounterVec
	completed  *prometheus.CounterVec
	retries    *prometheus.CounterVec
	failures   *prometheus.CounterVec
	processing prometheus.Histogram
	endToEnd   prometheus.Histogram
	probes     *prometheus.CounterVec
	mu         sync.Mutex
	bound      bool
}

func NewRecorder() *Recorder {
	r := &Recorder{registry: prometheus.NewRegistry()}
	r.received = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "mtb_demo_orders_received_total", Help: "Orders accepted or rejected."}, []string{"result"})
	r.completed = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "mtb_demo_orders_completed_total", Help: "Terminal order outcomes."}, []string{"result"})
	r.retries = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "mtb_demo_order_retries_total", Help: "Bounded processing retries."}, []string{"reason"})
	r.failures = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "mtb_demo_order_failures_total", Help: "Bounded terminal failures."}, []string{"reason"})
	r.processing = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "mtb_demo_order_processing_duration_seconds", Help: "Order processing duration.", Buckets: []float64{.05, .1, .25, .5, 1, 2, 5}})
	r.endToEnd = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "mtb_demo_order_end_to_end_duration_seconds", Help: "Order end-to-end duration.", Buckets: []float64{.1, .25, .5, 1, 2, 5, 10, 30}})
	r.probes = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "mtb_demo_business_probe_total", Help: "Bounded order probe outcomes."}, []string{"result"})
	r.registry.MustRegister(r.received, r.completed, r.retries, r.failures, r.processing, r.endToEnd, r.probes)
	for _, result := range []string{"accepted", "rejected"} {
		r.received.WithLabelValues(result)
	}
	for _, result := range []string{"completed", "failed"} {
		r.completed.WithLabelValues(result)
	}
	for _, result := range []string{"completed", "failed", "timed_out"} {
		r.probes.WithLabelValues(result)
	}
	return r
}

func (r *Recorder) BindEngine(engine *application.Engine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.bound {
		panic("prometheus recorder already bound")
	}
	r.bound = true
	r.registry.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "mtb_demo_order_queue_depth", Help: "Current queued order count."}, func() float64 { return float64(engine.QueueSnapshot().Depth) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "mtb_demo_order_queue_oldest_age_seconds", Help: "Age of the oldest queued order."}, func() float64 { return engine.QueueSnapshot().OldestAgeSeconds }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "mtb_demo_order_queue_capacity", Help: "Fixed queue capacity."}, func() float64 { return application.QueueCapacity }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "mtb_demo_worker_configured", Help: "Configured worker concurrency."}, func() float64 { return float64(engine.WorkerSnapshot().ConfiguredConcurrency) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "mtb_demo_worker_active", Help: "Active worker goroutines."}, func() float64 { return float64(engine.WorkerSnapshot().ActiveWorkers) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "mtb_demo_worker_inflight", Help: "Orders currently processing."}, func() float64 { return float64(engine.WorkerSnapshot().InflightOrders) }),
	)
}

func (r *Recorder) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{})
}
func (r *Recorder) Gatherer() prometheus.Gatherer         { return r.registry }
func (r *Recorder) OrderReceived(result string)           { r.received.WithLabelValues(result).Inc() }
func (r *Recorder) OrderCompleted(result string)          { r.completed.WithLabelValues(result).Inc() }
func (r *Recorder) OrderRetried(reason string)            { r.retries.WithLabelValues(reason).Inc() }
func (r *Recorder) OrderFailed(reason string)             { r.failures.WithLabelValues(reason).Inc() }
func (r *Recorder) ObserveProcessing(value time.Duration) { r.processing.Observe(value.Seconds()) }
func (r *Recorder) ObserveEndToEnd(value time.Duration)   { r.endToEnd.Observe(value.Seconds()) }
func (r *Recorder) BusinessProbe(result string)           { r.probes.WithLabelValues(result).Inc() }
