package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/servicemesh/virtualization-framework/internal/metrics"
)

func TestCollector_Observe(t *testing.T) {
	c := metrics.New()
	reg := prometheus.NewRegistry()
	c.MustRegister(reg)

	c.ObserveReconcile(metrics.ResultSuccess, 0.01)
	c.ObservePhase(simPhaseReady())
	c.SetGenerated("poc", "ref", 7)
	c.ObserveAdmissionDenied("validation")
	c.ObserveAdmissionDenied("")

	if got := testutil.ToFloat64(c.ReconcileTotal.WithLabelValues(metrics.ResultSuccess)); got != 1 {
		t.Fatalf("reconcile total = %v", got)
	}
	if got := testutil.ToFloat64(c.PhaseTransitions.WithLabelValues("Ready")); got != 1 {
		t.Fatalf("phase = %v", got)
	}
	if got := testutil.ToFloat64(c.GeneratedObjects.WithLabelValues("poc", "ref")); got != 7 {
		t.Fatalf("generated = %v", got)
	}
	if got := testutil.ToFloat64(c.AdmissionDenied.WithLabelValues("validation")); got != 1 {
		t.Fatalf("admission = %v", got)
	}
	if got := testutil.ToFloat64(c.AdmissionDenied.WithLabelValues("unknown")); got != 1 {
		t.Fatalf("unknown admission = %v", got)
	}
}

func TestNilSafe(t *testing.T) {
	var c *metrics.Collector
	c.ObserveReconcile(metrics.ResultError, 1)
	c.ObservePhase("")
	c.ObservePhase("Error")
	c.SetGenerated("a", "b", 1)
	c.ObserveAdmissionDenied("x")
}

func TestDefaultCollector(t *testing.T) {
	// Default is package-level; exercising Observe on Default is enough.
	metrics.Default.ObserveReconcile(metrics.ResultRequeue, 0.001)
}

func simPhaseReady() string { return "Ready" }
