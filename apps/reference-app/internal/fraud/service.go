package fraud

import (
	"context"
	"log/slog"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
	"github.com/servicemesh/reference-app/internal/sim"
)

// DeclineThreshold: scores at or above this map to DECLINE.
const DeclineThreshold int32 = 70

// Checker is the fraud evaluation use-case (Service B).
type Checker struct {
	log      *slog.Logger
	resolver RiskResolver
}

// NewChecker constructs a Checker. log may be nil.
func NewChecker(log *slog.Logger, resolver RiskResolver) *Checker {
	if log == nil {
		log = slog.Default()
	}
	return &Checker{log: log, resolver: resolver}
}

// CheckFraud resolves a risk evaluator, calls EvaluateRisk, maps score → recommendation.
func (c *Checker) CheckFraud(ctx context.Context, req *fraudv1.FraudCheckRequest) (*fraudv1.FraudCheckResponse, error) {
	scenario := sim.ScenarioFromContext(ctx)

	resolved, err := c.resolver.Resolve(ctx)
	if err != nil {
		c.log.Error("dial risk service failed", "err", err)
		return &fraudv1.FraudCheckResponse{
			TransactionId:  req.GetTransactionId(),
			Recommendation: "DECLINE",
			RiskScore:      100,
			Reason:         "RISK_SERVICE_UNAVAILABLE",
		}, nil
	}

	c.log.Info("CheckFraud",
		"txn", req.GetTransactionId(),
		"card", req.GetCardToken(),
		"simulation", scenario,
		"target", resolved.Target,
	)

	riskResp, err := resolved.Client.EvaluateRisk(ctx, &riskv1.RiskRequest{
		CardToken:   req.GetCardToken(),
		AmountCents: req.GetAmountCents(),
	})
	if err != nil {
		c.log.Error("EvaluateRisk failed", "err", err, "target", resolved.Target)
		return &fraudv1.FraudCheckResponse{
			TransactionId:  req.GetTransactionId(),
			Recommendation: "DECLINE",
			RiskScore:      100,
			Reason:         "RISK_SERVICE_ERROR",
		}, nil
	}

	rec, reason := MapRiskToRecommendation(riskResp)
	c.log.Info("fraud decision",
		"txn", req.GetTransactionId(),
		"risk_score", riskResp.GetRiskScore(),
		"recommendation", rec,
	)

	return &fraudv1.FraudCheckResponse{
		TransactionId:  req.GetTransactionId(),
		Recommendation: rec,
		RiskScore:      riskResp.GetRiskScore(),
		Reason:         reason,
	}, nil
}

// MapRiskToRecommendation converts a RiskResponse into APPROVE|DECLINE.
func MapRiskToRecommendation(riskResp *riskv1.RiskResponse) (rec, reason string) {
	rec = "APPROVE"
	reason = ""
	if riskResp.GetRiskScore() >= DeclineThreshold || riskResp.GetDecision() == "DECLINE" {
		rec = "DECLINE"
		reason = "HIGH_RISK_SCORE"
	}
	return rec, reason
}
