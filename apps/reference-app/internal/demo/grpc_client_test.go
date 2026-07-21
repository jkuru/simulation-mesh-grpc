package demo_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	paymentv1 "github.com/servicemesh/reference-app/gen/payment/v1"
	"github.com/servicemesh/reference-app/internal/demo"
)

type stubPayClient struct {
	resp *paymentv1.PaymentResponse
}

func (s *stubPayClient) ProcessPayment(ctx context.Context, in *paymentv1.PaymentRequest, opts ...grpc.CallOption) (*paymentv1.PaymentResponse, error) {
	return s.resp, nil
}

func TestGRPCPaymentGateway_Adapter(t *testing.T) {
	stub := &stubPayClient{resp: &paymentv1.PaymentResponse{Status: "APPROVED"}}
	adapter := demo.GRPCPaymentGateway{Client: stub}
	resp, err := adapter.ProcessPayment(context.Background(), &paymentv1.PaymentRequest{})
	if err != nil || resp.Status != "APPROVED" {
		t.Fatalf("%+v %v", resp, err)
	}
}
