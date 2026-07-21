package fraud

import (
	"context"

	"google.golang.org/grpc"

	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
)

// GRPCRiskEvaluator adapts riskv1.RiskServiceClient to the RiskEvaluator port.
type GRPCRiskEvaluator struct {
	Client riskv1.RiskServiceClient
	Opts   []grpc.CallOption
}

// EvaluateRisk implements RiskEvaluator.
func (c GRPCRiskEvaluator) EvaluateRisk(ctx context.Context, req *riskv1.RiskRequest) (*riskv1.RiskResponse, error) {
	return c.Client.EvaluateRisk(ctx, req, c.Opts...)
}
