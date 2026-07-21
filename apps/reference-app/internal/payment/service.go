package payment

import (
	"context"
	"log/slog"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
	paymentv1 "github.com/servicemesh/reference-app/gen/payment/v1"
	"github.com/servicemesh/reference-app/internal/sim"
)

// Gateway is the payment authorization use-case (Service A).
//
// It never interprets simulation scenario names for product decisions —
// only logs them and relies on FraudChecker (which propagates the header).
type Gateway struct {
	log   *slog.Logger
	fraud FraudChecker
	auth  AuthCodeGenerator
}

// NewGateway constructs a Gateway. Nil logger/auth get safe defaults.
func NewGateway(log *slog.Logger, fraud FraudChecker, auth AuthCodeGenerator) *Gateway {
	if log == nil {
		log = slog.Default()
	}
	if auth == nil {
		auth = RandomAuthCode{}
	}
	return &Gateway{log: log, fraud: fraud, auth: auth}
}

// ProcessPayment orchestrates fraud check then approves or declines.
func (g *Gateway) ProcessPayment(ctx context.Context, req *paymentv1.PaymentRequest) (*paymentv1.PaymentResponse, error) {
	scenario := sim.ScenarioFromContext(ctx)
	g.log.Info("ProcessPayment",
		"txn", req.GetTransactionId(),
		"card", req.GetCardToken(),
		"amount_cents", req.GetAmountCents(),
		"simulation", scenario,
	)

	fraudResp, err := g.fraud.CheckFraud(ctx, &fraudv1.FraudCheckRequest{
		TransactionId: req.GetTransactionId(),
		CardToken:     req.GetCardToken(),
		AmountCents:   req.GetAmountCents(),
	})
	if err != nil {
		g.log.Error("fraud check failed", "err", err)
		return &paymentv1.PaymentResponse{
			TransactionId: req.GetTransactionId(),
			Status:        "DECLINED",
			DeclineReason: "FRAUD_CHECK_UNAVAILABLE",
		}, nil
	}

	if fraudResp.GetRecommendation() == "DECLINE" {
		g.log.Info("payment DECLINED", "txn", req.GetTransactionId(), "reason", fraudResp.GetReason())
		return &paymentv1.PaymentResponse{
			TransactionId:  req.GetTransactionId(),
			Status:         "DECLINED",
			DeclineReason:  fraudResp.GetReason(),
			RiskScore:      fraudResp.GetRiskScore(),
			Recommendation: fraudResp.GetRecommendation(),
		}, nil
	}

	auth := g.auth.Generate()
	g.log.Info("payment APPROVED", "txn", req.GetTransactionId(), "auth", auth)
	return &paymentv1.PaymentResponse{
		TransactionId:  req.GetTransactionId(),
		Status:         "APPROVED",
		AuthCode:       auth,
		RiskScore:      fraudResp.GetRiskScore(),
		Recommendation: fraudResp.GetRecommendation(),
	}, nil
}
