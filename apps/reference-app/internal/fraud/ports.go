// Package fraud implements Service B (fraud-checker) with interface-first design.
//
// Simulation local-mode routing is expressed as a RiskResolver port so tests
// assert routing without opening real gRPC connections.
package fraud

import (
	"context"

	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
)

// RiskEvaluator evaluates card risk (third party or virtual backend).
// Matches the surface of riskv1.RiskServiceClient.EvaluateRisk.
type RiskEvaluator interface {
	EvaluateRisk(ctx context.Context, req *riskv1.RiskRequest) (*riskv1.RiskResponse, error)
}

// ResolvedRisk is a chosen evaluator plus a log label (endpoint or "fake").
type ResolvedRisk struct {
	Client RiskEvaluator
	Target string
}

// RiskResolver chooses which RiskEvaluator to use for a request.
//
// Production local mode: LocalRiskResolver (header → Microcks).
// Production mesh mode: FixedRiskResolver (always external; Istio redirects).
// Tests: function fakes.
type RiskResolver interface {
	Resolve(ctx context.Context) (ResolvedRisk, error)
}
