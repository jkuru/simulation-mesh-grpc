package admission

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/config"
)

// Validator is a validating admission webhook for SimulationManifest.
type Validator struct {
	Config config.Config
}

var _ admission.CustomValidator = &Validator{}

// ValidateCreate implements admission.CustomValidator.
func (v *Validator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	m, err := asManifest(obj)
	if err != nil {
		return nil, err
	}
	if err := ValidateForAdmission(m, v.Config); err != nil {
		return nil, fmt.Errorf("SimulationManifest denied: %w", err)
	}
	return nil, nil
}

// ValidateUpdate implements admission.CustomValidator.
func (v *Validator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	m, err := asManifest(newObj)
	if err != nil {
		return nil, err
	}
	if err := ValidateForAdmission(m, v.Config); err != nil {
		return nil, fmt.Errorf("SimulationManifest denied: %w", err)
	}
	return nil, nil
}

// ValidateDelete always allows deletion (cleanup must not be blocked by policy).
func (v *Validator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func asManifest(obj runtime.Object) (*simv1.SimulationManifest, error) {
	m, ok := obj.(*simv1.SimulationManifest)
	if !ok {
		return nil, fmt.Errorf("expected SimulationManifest, got %T", obj)
	}
	return m, nil
}

// Note: webhook registration (NewWebhookManagedBy) lives in cmd/operator.
