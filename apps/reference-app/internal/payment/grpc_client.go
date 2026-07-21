package payment

import (
	"context"

	"google.golang.org/grpc"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
)

// GRPCFraudChecker adapts fraudv1.FraudCheckerClient to the FraudChecker port.
// Generated clients take optional grpc.CallOption args, so they do not satisfy
// FraudChecker directly — this adapter is the composition-root glue.
type GRPCFraudChecker struct {
	Client fraudv1.FraudCheckerClient
	Opts   []grpc.CallOption
}

// CheckFraud implements FraudChecker.
func (c GRPCFraudChecker) CheckFraud(ctx context.Context, req *fraudv1.FraudCheckRequest) (*fraudv1.FraudCheckResponse, error) {
	return c.Client.CheckFraud(ctx, req, c.Opts...)
}
