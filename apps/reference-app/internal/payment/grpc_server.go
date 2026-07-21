package payment

import (
	"context"

	paymentv1 "github.com/servicemesh/reference-app/gen/payment/v1"
)

// GRPCServer adapts Gateway to the generated PaymentGatewayServer interface.
type GRPCServer struct {
	paymentv1.UnimplementedPaymentGatewayServer
	Gateway *Gateway
}

// ProcessPayment delegates to Gateway.
func (s *GRPCServer) ProcessPayment(ctx context.Context, req *paymentv1.PaymentRequest) (*paymentv1.PaymentResponse, error) {
	return s.Gateway.ProcessPayment(ctx, req)
}
