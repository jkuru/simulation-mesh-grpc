package checkout

import (
	"context"

	checkoutv1 "github.com/servicemesh/reference-app/gen/checkout/v1"
)

// GRPCServer adapts Gateway to the generated CheckoutGatewayServer interface.
type GRPCServer struct {
	checkoutv1.UnimplementedCheckoutGatewayServer
	Gateway *Gateway
}

// ProcessCheckout delegates to Gateway.
func (s *GRPCServer) ProcessCheckout(ctx context.Context, req *checkoutv1.CheckoutRequest) (*checkoutv1.CheckoutResponse, error) {
	return s.Gateway.ProcessCheckout(ctx, req)
}
