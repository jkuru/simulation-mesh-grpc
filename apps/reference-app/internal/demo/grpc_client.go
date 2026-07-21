package demo

import (
	"context"

	"google.golang.org/grpc"

	checkoutv1 "github.com/servicemesh/reference-app/gen/checkout/v1"
)

// GRPCCheckoutGateway adapts checkoutv1.CheckoutGatewayClient to CheckoutGateway.
type GRPCCheckoutGateway struct {
	Client checkoutv1.CheckoutGatewayClient
	Opts   []grpc.CallOption
}

// ProcessCheckout implements CheckoutGateway.
func (c GRPCCheckoutGateway) ProcessCheckout(ctx context.Context, req *checkoutv1.CheckoutRequest) (*checkoutv1.CheckoutResponse, error) {
	return c.Client.ProcessCheckout(ctx, req, c.Opts...)
}
