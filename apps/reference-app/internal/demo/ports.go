// Package demo is the test-client use-case: prove header-driven virtualization.
package demo

import (
	"context"

	paymentv1 "github.com/servicemesh/reference-app/gen/payment/v1"
)

// PaymentGateway is the outbound dependency of the demo runner.
type PaymentGateway interface {
	ProcessPayment(ctx context.Context, req *paymentv1.PaymentRequest) (*paymentv1.PaymentResponse, error)
}

// Case is one payment attempt in the demo.
type Case struct {
	Label    string
	Scenario string // empty = no simulation header
	Txn      string
}

// Result is the outcome of one Case.
type Result struct {
	Case    Case
	Resp    *paymentv1.PaymentResponse
	Err     error
	Elapsed int64 // nanoseconds; wall clock filled by Runner
}
