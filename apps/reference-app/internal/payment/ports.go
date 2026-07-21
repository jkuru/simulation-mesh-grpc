// Package payment implements Service A (payment-gateway) with an
// interface-first design so unit tests inject fakes instead of gRPC.
//
// Ports (interfaces) are defined here — the consumer package owns them.
// Concrete gRPC adapters live in cmd/payment-gateway (composition root).
package payment

import (
	"context"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
)

// FraudChecker is the outbound dependency used by the payment gateway.
// Production wiring uses fraudv1.FraudCheckerClient; tests use fakes.
type FraudChecker interface {
	CheckFraud(ctx context.Context, req *fraudv1.FraudCheckRequest) (*fraudv1.FraudCheckResponse, error)
}

// AuthCodeGenerator produces approval auth codes (injectable for deterministic tests).
type AuthCodeGenerator interface {
	Generate() string
}
