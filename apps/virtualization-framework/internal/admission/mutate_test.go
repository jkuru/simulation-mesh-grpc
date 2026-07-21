package admission_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/servicemesh/virtualization-contract"
	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/admission"
	"github.com/servicemesh/virtualization-framework/internal/config"
)

func TestApplyDefaults(t *testing.T) {
	m := &simv1.SimulationManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "poc"},
		Spec:       simv1.SimulationManifestSpec{},
	}
	cfg := config.Config{DefaultMicrocksHostPort: "mock.svc:9090"}
	admission.ApplyDefaults(m, cfg)
	if m.Spec.MicrocksService != "mock.svc:9090" {
		t.Fatalf("microcksService=%q", m.Spec.MicrocksService)
	}
	if m.Spec.VirtualBackend != contract.BackendTeachingMock {
		t.Fatalf("virtualBackend=%q", m.Spec.VirtualBackend)
	}

	// does not overwrite
	m.Spec.MicrocksService = "custom:1"
	m.Spec.VirtualBackend = contract.BackendMicrocks
	admission.ApplyDefaults(m, cfg)
	if m.Spec.MicrocksService != "custom:1" || m.Spec.VirtualBackend != contract.BackendMicrocks {
		t.Fatal("should preserve user values")
	}
}

func TestApplyDefaults_NilAndEmptyConfig(t *testing.T) {
	admission.ApplyDefaults(nil, config.Config{})
	m := &simv1.SimulationManifest{}
	admission.ApplyDefaults(m, config.Config{})
	if m.Spec.MicrocksService != contract.DefaultMicrocksHostPort {
		t.Fatalf("got %q", m.Spec.MicrocksService)
	}
}

func TestDefaulter_Default(t *testing.T) {
	d := &admission.Defaulter{Config: config.Config{DefaultMicrocksHostPort: "a:1"}}
	m := &simv1.SimulationManifest{}
	if err := d.Default(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if m.Spec.MicrocksService != "a:1" {
		t.Fatal(m.Spec.MicrocksService)
	}
	if err := d.Default(context.Background(), &simv1.SimulationManifestList{}); err == nil {
		t.Fatal("expected type error")
	}
}
