package risk

import (
	"context"

	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
)

// RiskEvaluator is the shared use-case surface for both backends.
// (Alias of the method set; both ExternalService and VirtualService implement it.)
type RiskHandler interface {
	EvaluateRisk(ctx context.Context, req *riskv1.RiskRequest) (*riskv1.RiskResponse, error)
}

// GRPCServer adapts a RiskHandler to the generated RiskServiceServer.
type GRPCServer struct {
	riskv1.UnimplementedRiskServiceServer
	Handler RiskHandler
}

// EvaluateRisk delegates to Handler.
func (s *GRPCServer) EvaluateRisk(ctx context.Context, req *riskv1.RiskRequest) (*riskv1.RiskResponse, error) {
	return s.Handler.EvaluateRisk(ctx, req)
}
