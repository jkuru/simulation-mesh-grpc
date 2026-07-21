// Package metrics exposes Prometheus instrumentation for the virtualization-framework operator.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Result labels for ReconcileTotal.
	ResultSuccess   = "success"
	ResultError     = "error"
	ResultForbidden = "forbidden"
	ResultDeleted   = "deleted"
	ResultRequeue   = "requeue"
)

// Collector holds named metrics. Tests may construct an isolated Collector.
type Collector struct {
	ReconcileTotal    *prometheus.CounterVec
	ReconcileDuration *prometheus.HistogramVec
	PhaseTransitions  *prometheus.CounterVec
	GeneratedObjects  *prometheus.GaugeVec
	AdmissionDenied   *prometheus.CounterVec
}

// New creates a Collector with fresh metric vectors (not registered).
func New() *Collector {
	return &Collector{
		ReconcileTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "virtualization_framework",
			Name:      "reconcile_total",
			Help:      "Total SimulationManifest reconciles by result",
		}, []string{"result"}),
		ReconcileDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "virtualization_framework",
			Name:      "reconcile_duration_seconds",
			Help:      "SimulationManifest reconcile duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"result"}),
		PhaseTransitions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "virtualization_framework",
			Name:      "phase_transitions_total",
			Help:      "SimulationManifest status phase transitions",
		}, []string{"phase"}),
		GeneratedObjects: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "virtualization_framework",
			Name:      "generated_objects",
			Help:      "Number of generated Istio objects for a manifest (last reconcile)",
		}, []string{"namespace", "name"}),
		AdmissionDenied: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "virtualization_framework",
			Name:      "admission_denied_total",
			Help:      "SimulationManifest admission denials by reason class",
		}, []string{"reason"}),
	}
}

// MustRegister registers all metrics on the given registerer (e.g. metrics.Registry).
func (c *Collector) MustRegister(r prometheus.Registerer) {
	r.MustRegister(
		c.ReconcileTotal,
		c.ReconcileDuration,
		c.PhaseTransitions,
		c.GeneratedObjects,
		c.AdmissionDenied,
	)
}

// Default is the process-wide collector used by the operator.
// cmd/operator registers it on controller-runtime's metrics.Registry.
var Default = New()

// ObserveReconcile records duration and result counters.
func (c *Collector) ObserveReconcile(result string, seconds float64) {
	if c == nil {
		return
	}
	c.ReconcileTotal.WithLabelValues(result).Inc()
	c.ReconcileDuration.WithLabelValues(result).Observe(seconds)
}

// ObservePhase increments phase transition counter.
func (c *Collector) ObservePhase(phase string) {
	if c == nil || phase == "" {
		return
	}
	c.PhaseTransitions.WithLabelValues(phase).Inc()
}

// SetGenerated records how many objects the last successful generate produced.
func (c *Collector) SetGenerated(namespace, name string, n float64) {
	if c == nil {
		return
	}
	c.GeneratedObjects.WithLabelValues(namespace, name).Set(n)
}

// ObserveAdmissionDenied increments admission denial counter.
func (c *Collector) ObserveAdmissionDenied(reason string) {
	if c == nil {
		return
	}
	if reason == "" {
		reason = "unknown"
	}
	c.AdmissionDenied.WithLabelValues(reason).Inc()
}
