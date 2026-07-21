package checkout_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
	"github.com/servicemesh/reference-app/internal/checkout"
)

type stubFraudClient struct {
	resp *fraudv1.FraudCheckResponse
	err  error
	opts int
}

func (s *stubFraudClient) CheckFraud(ctx context.Context, in *fraudv1.FraudCheckRequest, opts ...grpc.CallOption) (*fraudv1.FraudCheckResponse, error) {
	s.opts = len(opts)
	return s.resp, s.err
}

func TestGRPCFraudChecker_Adapter(t *testing.T) {
	stub := &stubFraudClient{resp: &fraudv1.FraudCheckResponse{Recommendation: "APPROVE"}}
	adapter := checkout.GRPCFraudChecker{Client: stub, Opts: []grpc.CallOption{grpc.WaitForReady(true)}}
	resp, err := adapter.CheckFraud(context.Background(), &fraudv1.FraudCheckRequest{TransactionId: "t"})
	if err != nil || resp.Recommendation != "APPROVE" || stub.opts != 1 {
		t.Fatalf("resp=%+v err=%v opts=%d", resp, err, stub.opts)
	}
}
