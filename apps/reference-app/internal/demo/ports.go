// Package demo is the test-client use-case: prove header-driven virtualization.
package demo

import (
	"context"

	checkoutv1 "github.com/servicemesh/reference-app/gen/checkout/v1"
)

// CheckoutGateway is the outbound dependency of the demo runner.
type CheckoutGateway interface {
	ProcessCheckout(ctx context.Context, req *checkoutv1.CheckoutRequest) (*checkoutv1.CheckoutResponse, error)
}

// Case is one checkout attempt in the demo.
type Case struct {
	Label    string
	Scenario string // empty = no simulation header
	Txn      string
}

// Result is the outcome of one Case.
type Result struct {
	Case    Case
	Resp    *checkoutv1.CheckoutResponse
	Err     error
	Elapsed int64 // nanoseconds; wall clock filled by Runner
}
