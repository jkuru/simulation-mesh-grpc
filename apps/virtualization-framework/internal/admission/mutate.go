package admission

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/servicemesh/virtualization-contract"
	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/config"
)

// Defaulter is a mutating admission webhook that fills SimulationManifest defaults.
type Defaulter struct {
	Config config.Config
}

// Default implements admission.CustomDefaulter.
func (d *Defaulter) Default(_ context.Context, obj runtime.Object) error {
	m, ok := obj.(*simv1.SimulationManifest)
	if !ok {
		return fmt.Errorf("expected SimulationManifest, got %T", obj)
	}
	ApplyDefaults(m, d.Config)
	return nil
}

// ApplyDefaults mutates m in place with platform defaults (also usable in unit tests).
func ApplyDefaults(m *simv1.SimulationManifest, cfg config.Config) {
	if m == nil {
		return
	}
	if strings.TrimSpace(m.Spec.MicrocksService) == "" {
		def := strings.TrimSpace(cfg.DefaultMicrocksHostPort)
		if def == "" {
			def = contract.DefaultMicrocksHostPort
		}
		m.Spec.MicrocksService = def
	}
	switch strings.TrimSpace(m.Spec.VirtualBackend) {
	case "":
		m.Spec.VirtualBackend = contract.BackendTeachingMock
	case contract.BackendTeachingMock, contract.BackendMicrocks:
		// already valid
	default:
		// leave as-is; validating webhook / OpenAPI reject unknown enums
	}
}
