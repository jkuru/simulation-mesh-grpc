// Package checkout implements Service A (checkout-gateway) with an
// interface-first design so unit tests inject fakes instead of gRPC.
//
// Ports (interfaces) are defined here — the consumer package owns them.
// Concrete gRPC adapters live in cmd/checkout-gateway (composition root).
package checkout

import (
	"context"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
)

// FraudChecker is the outbound dependency used by the checkout gateway.
// Production wiring uses fraudv1.FraudCheckerClient; tests use fakes.
type FraudChecker interface {
	CheckFraud(ctx context.Context, req *fraudv1.FraudCheckRequest) (*fraudv1.FraudCheckResponse, error)
}

// OrderCodeGenerator produces approval order codes (injectable for deterministic tests).
type OrderCodeGenerator interface {
	Generate() string
}
