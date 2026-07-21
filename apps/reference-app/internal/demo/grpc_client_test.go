package demo_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	checkoutv1 "github.com/servicemesh/reference-app/gen/checkout/v1"
	"github.com/servicemesh/reference-app/internal/demo"
)

type stubPayClient struct {
	resp *checkoutv1.CheckoutResponse
}

func (s *stubPayClient) ProcessCheckout(ctx context.Context, in *checkoutv1.CheckoutRequest, opts ...grpc.CallOption) (*checkoutv1.CheckoutResponse, error) {
	return s.resp, nil
}

func TestGRPCCheckoutGateway_Adapter(t *testing.T) {
	stub := &stubPayClient{resp: &checkoutv1.CheckoutResponse{Status: "APPROVED"}}
	adapter := demo.GRPCCheckoutGateway{Client: stub}
	resp, err := adapter.ProcessCheckout(context.Background(), &checkoutv1.CheckoutRequest{})
	if err != nil || resp.Status != "APPROVED" {
		t.Fatalf("%+v %v", resp, err)
	}
}
