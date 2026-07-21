// Package events defines stable Kubernetes Event reasons for the operator.
package events

// Event reasons (short, CamelCase) — shown in kubectl describe / get events.
const (
	ReasonReady           = "Ready"
	ReasonReconcileError  = "ReconcileError"
	ReasonForbidden       = "Forbidden"
	ReasonDeleting        = "Deleting"
	ReasonFinalizerAdded  = "FinalizerAdded"
	ReasonValidationError = "ValidationError"
)
