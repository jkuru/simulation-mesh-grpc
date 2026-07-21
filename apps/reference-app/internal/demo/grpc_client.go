package demo

import (
	"context"

	"google.golang.org/grpc"

	paymentv1 "github.com/servicemesh/reference-app/gen/payment/v1"
)

// GRPCPaymentGateway adapts paymentv1.PaymentGatewayClient to PaymentGateway.
type GRPCPaymentGateway struct {
	Client paymentv1.PaymentGatewayClient
	Opts   []grpc.CallOption
}

// ProcessPayment implements PaymentGateway.
func (c GRPCPaymentGateway) ProcessPayment(ctx context.Context, req *paymentv1.PaymentRequest) (*paymentv1.PaymentResponse, error) {
	return c.Client.ProcessPayment(ctx, req, c.Opts...)
}
