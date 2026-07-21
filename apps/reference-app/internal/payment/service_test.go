package payment_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
	paymentv1 "github.com/servicemesh/reference-app/gen/payment/v1"
	"github.com/servicemesh/reference-app/internal/payment"
	"github.com/servicemesh/reference-app/internal/sim"
)

// fakeFraud implements payment.FraudChecker for tests.
type fakeFraud struct {
	resp *fraudv1.FraudCheckResponse
	err  error
	// lastCtx captures scenario propagation expectations.
	lastScenario string
}

func (f *fakeFraud) CheckFraud(ctx context.Context, req *fraudv1.FraudCheckRequest) (*fraudv1.FraudCheckResponse, error) {
	f.lastScenario = sim.ScenarioFromContext(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func silentLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestProcessPayment_Approved(t *testing.T) {
	ff := &fakeFraud{resp: &fraudv1.FraudCheckResponse{
		Recommendation: "APPROVE", RiskScore: 10,
	}}
	gw := payment.NewGateway(silentLog(), ff, payment.FixedAuthCode{Code: "AUTH-TEST-X"})

	resp, err := gw.ProcessPayment(context.Background(), &paymentv1.PaymentRequest{
		TransactionId: "t1", CardToken: "tok_low_risk_1", AmountCents: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "APPROVED" || resp.AuthCode != "AUTH-TEST-X" || resp.RiskScore != 10 {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

func TestProcessPayment_DeclinedByFraud(t *testing.T) {
	ff := &fakeFraud{resp: &fraudv1.FraudCheckResponse{
		Recommendation: "DECLINE", RiskScore: 92, Reason: "HIGH_RISK_SCORE",
	}}
	gw := payment.NewGateway(silentLog(), ff, payment.FixedAuthCode{Code: "NOPE"})

	resp, err := gw.ProcessPayment(context.Background(), &paymentv1.PaymentRequest{
		TransactionId: "t2", CardToken: "tok_low_risk_1", AmountCents: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "DECLINED" || resp.DeclineReason != "HIGH_RISK_SCORE" {
		t.Fatalf("unexpected resp: %+v", resp)
	}
	if resp.AuthCode != "" {
		t.Fatalf("auth code should be empty on decline, got %q", resp.AuthCode)
	}
}

func TestProcessPayment_FraudUnavailable(t *testing.T) {
	ff := &fakeFraud{err: errors.New("boom")}
	gw := payment.NewGateway(silentLog(), ff, nil)

	resp, err := gw.ProcessPayment(context.Background(), &paymentv1.PaymentRequest{
		TransactionId: "t3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "DECLINED" || resp.DeclineReason != "FRAUD_CHECK_UNAVAILABLE" {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

func TestProcessPayment_PropagatesScenarioInContext(t *testing.T) {
	ff := &fakeFraud{resp: &fraudv1.FraudCheckResponse{Recommendation: "APPROVE"}}
	gw := payment.NewGateway(silentLog(), ff, payment.FixedAuthCode{Code: "A"})

	ctx := sim.WithScenario(context.Background(), "fraud-declined")
	_, _ = gw.ProcessPayment(ctx, &paymentv1.PaymentRequest{TransactionId: "t4"})
	if ff.lastScenario != "fraud-declined" {
		t.Fatalf("scenario = %q", ff.lastScenario)
	}
}

func TestGRPCServer_Delegates(t *testing.T) {
	ff := &fakeFraud{resp: &fraudv1.FraudCheckResponse{Recommendation: "APPROVE", RiskScore: 5}}
	gw := payment.NewGateway(silentLog(), ff, payment.FixedAuthCode{Code: "AUTH-1"})
	srv := &payment.GRPCServer{Gateway: gw}

	resp, err := srv.ProcessPayment(context.Background(), &paymentv1.PaymentRequest{TransactionId: "g1"})
	if err != nil || resp.GetStatus() != "APPROVED" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
}

func TestNewGateway_Defaults(t *testing.T) {
	ff := &fakeFraud{resp: &fraudv1.FraudCheckResponse{Recommendation: "APPROVE"}}
	gw := payment.NewGateway(nil, ff, nil)
	resp, err := gw.ProcessPayment(context.Background(), &paymentv1.PaymentRequest{TransactionId: "d1"})
	if err != nil || resp.GetStatus() != "APPROVED" || resp.GetAuthCode() == "" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
}

func TestRandomAuthCode_Generate(t *testing.T) {
	code := payment.RandomAuthCode{}.Generate()
	if len(code) < 8 {
		t.Fatalf("code too short: %q", code)
	}
}

func TestRandomAuthCode_WithRand(t *testing.T) {
	// Deterministic: seed 1
	a := payment.RandomAuthCode{Rand: newRandSource(1)}.Generate()
	b := payment.RandomAuthCode{Rand: newRandSource(1)}.Generate()
	if a != b {
		t.Fatalf("expected deterministic codes, got %q vs %q", a, b)
	}
}
