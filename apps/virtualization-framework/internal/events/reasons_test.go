package events_test

import (
	"testing"

	"github.com/servicemesh/virtualization-framework/internal/events"
)

func TestReasonsNonEmpty(t *testing.T) {
	rs := []string{
		events.ReasonReady,
		events.ReasonReconcileError,
		events.ReasonForbidden,
		events.ReasonDeleting,
		events.ReasonFinalizerAdded,
		events.ReasonValidationError,
	}
	for _, r := range rs {
		if r == "" {
			t.Fatal("empty reason")
		}
	}
}
