package contract_test

import (
	"testing"

	"github.com/servicemesh/virtualization-contract"
)

func TestConstantsStable(t *testing.T) {
	// These names are the platform product contract — changing them is a breaking change.
	checks := map[string]string{
		"SimulationHeader":        contract.SimulationHeader,
		"MicrocksOperationHeader": contract.MicrocksOperationHeader,
		"PropagationLabelKey":     contract.PropagationLabelKey,
		"PropagationLabelValue":   contract.PropagationLabelValue,
		"ManagedByLabelValue":     contract.ManagedByLabelValue,
		"BackendTeachingMock":     contract.BackendTeachingMock,
		"BackendMicrocks":         contract.BackendMicrocks,
		"DefaultMicrocksHostPort": contract.DefaultMicrocksHostPort,
		"SystemNamespace":         contract.SystemNamespace,
		"EnvironmentKey":          contract.EnvironmentKey,
	}
	for name, v := range checks {
		if v == "" {
			t.Fatalf("%s empty", name)
		}
	}
	if contract.SimulationHeader != "test-data-simulation-action-name" {
		t.Fatalf("SimulationHeader drifted: %q", contract.SimulationHeader)
	}
	if contract.PropagationLabelValue != "enabled" {
		t.Fatalf("PropagationLabelValue drifted: %q", contract.PropagationLabelValue)
	}
}
