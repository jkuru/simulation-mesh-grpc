package fraud

import (
	"context"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
)

// GRPCServer adapts Checker to the generated FraudCheckerServer interface.
type GRPCServer struct {
	fraudv1.UnimplementedFraudCheckerServer
	Checker *Checker
}

// CheckFraud delegates to Checker.
func (s *GRPCServer) CheckFraud(ctx context.Context, req *fraudv1.FraudCheckRequest) (*fraudv1.FraudCheckResponse, error) {
	return s.Checker.CheckFraud(ctx, req)
}
