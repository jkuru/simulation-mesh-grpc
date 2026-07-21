package fraud_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
	"github.com/servicemesh/reference-app/internal/fraud"
)

type stubRiskClient struct {
	resp *riskv1.RiskResponse
}

func (s *stubRiskClient) EvaluateRisk(ctx context.Context, in *riskv1.RiskRequest, opts ...grpc.CallOption) (*riskv1.RiskResponse, error) {
	return s.resp, nil
}

func TestGRPCRiskEvaluator_Adapter(t *testing.T) {
	stub := &stubRiskClient{resp: &riskv1.RiskResponse{RiskScore: 3}}
	adapter := fraud.GRPCRiskEvaluator{Client: stub}
	resp, err := adapter.EvaluateRisk(context.Background(), &riskv1.RiskRequest{})
	if err != nil || resp.RiskScore != 3 {
		t.Fatalf("%+v %v", resp, err)
	}
}
