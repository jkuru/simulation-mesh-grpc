package checkout

import (
	"context"
	"log/slog"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
	checkoutv1 "github.com/servicemesh/reference-app/gen/checkout/v1"
	"github.com/servicemesh/reference-app/internal/sim"
)

// Gateway is the checkout authorization use-case (Service A).
//
// It never interprets simulation scenario names for product decisions —
// only logs them and relies on FraudChecker (which propagates the header).
type Gateway struct {
	log   *slog.Logger
	fraud FraudChecker
	auth  OrderCodeGenerator
}

// NewGateway constructs a Gateway. Nil logger/auth get safe defaults.
func NewGateway(log *slog.Logger, fraud FraudChecker, auth OrderCodeGenerator) *Gateway {
	if log == nil {
		log = slog.Default()
	}
	if auth == nil {
		auth = RandomOrderCode{}
	}
	return &Gateway{log: log, fraud: fraud, auth: auth}
}

// ProcessCheckout orchestrates fraud check then approves or declines.
func (g *Gateway) ProcessCheckout(ctx context.Context, req *checkoutv1.CheckoutRequest) (*checkoutv1.CheckoutResponse, error) {
	scenario := sim.ScenarioFromContext(ctx)
	g.log.Info("ProcessCheckout",
		"txn", req.GetTransactionId(),
		"nft", req.GetNftToken(),
		"amount_cents", req.GetAmountCents(),
		"simulation", scenario,
	)

	fraudResp, err := g.fraud.CheckFraud(ctx, &fraudv1.FraudCheckRequest{
		TransactionId: req.GetTransactionId(),
		NftToken:     req.GetNftToken(),
		AmountCents:   req.GetAmountCents(),
	})
	if err != nil {
		g.log.Error("fraud check failed", "err", err)
		return &checkoutv1.CheckoutResponse{
			TransactionId: req.GetTransactionId(),
			Status:        "DECLINED",
			DeclineReason: "FRAUD_CHECK_UNAVAILABLE",
		}, nil
	}

	if fraudResp.GetRecommendation() == "DECLINE" {
		g.log.Info("checkout DECLINED", "txn", req.GetTransactionId(), "reason", fraudResp.GetReason())
		return &checkoutv1.CheckoutResponse{
			TransactionId:  req.GetTransactionId(),
			Status:         "DECLINED",
			DeclineReason:  fraudResp.GetReason(),
			RiskScore:      fraudResp.GetRiskScore(),
			Recommendation: fraudResp.GetRecommendation(),
		}, nil
	}

	auth := g.auth.Generate()
	g.log.Info("checkout APPROVED", "txn", req.GetTransactionId(), "auth", auth)
	return &checkoutv1.CheckoutResponse{
		TransactionId:  req.GetTransactionId(),
		Status:         "APPROVED",
		OrderCode:       auth,
		RiskScore:      fraudResp.GetRiskScore(),
		Recommendation: fraudResp.GetRecommendation(),
	}, nil
}
