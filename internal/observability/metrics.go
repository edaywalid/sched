package observability

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics is the engine's Prometheus surface. Counters and histograms
// are registered against the default registry via promauto so any
// /metrics handler picks them up.
type Metrics struct {
	WorkflowsStarted   prometheus.Counter
	WorkflowsCompleted *prometheus.CounterVec // labels: status
	ActivitiesExecuted *prometheus.CounterVec // labels: status
	ActivityRetries    prometheus.Counter
	ActivityDuration   prometheus.Histogram
	TaskPollLatency    *prometheus.HistogramVec // labels: kind (workflow|activity)
}

// NewMetrics registers all engine metrics under the sched_ namespace.
// Safe to call exactly once per process; subsequent calls panic in
// promauto because the metrics are already registered.
func NewMetrics() *Metrics {
	return &Metrics{
		WorkflowsStarted: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "sched",
			Name:      "workflows_started_total",
			Help:      "Number of workflows accepted by StartWorkflow.",
		}),
		WorkflowsCompleted: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "sched",
			Name:      "workflows_completed_total",
			Help:      "Workflow terminal transitions, partitioned by final status.",
		}, []string{"status"}),
		ActivitiesExecuted: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "sched",
			Name:      "activities_executed_total",
			Help:      "Activity attempts that reported a result, partitioned by status.",
		}, []string{"status"}),
		ActivityRetries: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "sched",
			Name:      "activity_retries_total",
			Help:      "Number of activity retries scheduled.",
		}),
		ActivityDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "sched",
			Name:      "activity_duration_seconds",
			Help:      "Wall-clock time from PollActivityTask to CompleteActivity.",
			Buckets:   prometheus.ExponentialBuckets(0.05, 2, 12),
		}),
		TaskPollLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "sched",
			Name:      "task_poll_latency_seconds",
			Help:      "PollWorkflowTask / PollActivityTask wait time before a task is returned.",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 14),
		}, []string{"kind"}),
	}
}

// StartMetricsServer serves /metrics on the given port. The returned
// shutdown function should be called on engine exit. Errors from
// ListenAndServe (other than ErrServerClosed) are surfaced through
// errCh.
func StartMetricsServer(port int) (shutdown func(), errCh <-chan error) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	ch := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			ch <- err
		}
		close(ch)
	}()
	return func() { _ = server.Close() }, ch
}
