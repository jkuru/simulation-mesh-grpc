package checkout_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
	checkoutv1 "github.com/servicemesh/reference-app/gen/checkout/v1"
	"github.com/servicemesh/reference-app/internal/checkout"
	"github.com/servicemesh/reference-app/internal/sim"
)

// fakeFraud implements checkout.FraudChecker for tests.
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

func TestProcessCheckout_Approved(t *testing.T) {
	ff := &fakeFraud{resp: &fraudv1.FraudCheckResponse{
		Recommendation: "APPROVE", RiskScore: 10,
	}}
	gw := checkout.NewGateway(silentLog(), ff, checkout.FixedOrderCode{Code: "ORDER-TEST-X"})

	resp, err := gw.ProcessCheckout(context.Background(), &checkoutv1.CheckoutRequest{
		TransactionId: "t1", NftToken: "nft_low_risk_1", AmountCents: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "APPROVED" || resp.OrderCode != "ORDER-TEST-X" || resp.RiskScore != 10 {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

func TestProcessCheckout_DeclinedByFraud(t *testing.T) {
	ff := &fakeFraud{resp: &fraudv1.FraudCheckResponse{
		Recommendation: "DECLINE", RiskScore: 92, Reason: "HIGH_RISK_SCORE",
	}}
	gw := checkout.NewGateway(silentLog(), ff, checkout.FixedOrderCode{Code: "NOPE"})

	resp, err := gw.ProcessCheckout(context.Background(), &checkoutv1.CheckoutRequest{
		TransactionId: "t2", NftToken: "nft_low_risk_1", AmountCents: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "DECLINED" || resp.DeclineReason != "HIGH_RISK_SCORE" {
		t.Fatalf("unexpected resp: %+v", resp)
	}
	if resp.OrderCode != "" {
		t.Fatalf("auth code should be empty on decline, got %q", resp.OrderCode)
	}
}

func TestProcessCheckout_FraudUnavailable(t *testing.T) {
	ff := &fakeFraud{err: errors.New("boom")}
	gw := checkout.NewGateway(silentLog(), ff, nil)

	resp, err := gw.ProcessCheckout(context.Background(), &checkoutv1.CheckoutRequest{
		TransactionId: "t3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "DECLINED" || resp.DeclineReason != "FRAUD_CHECK_UNAVAILABLE" {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

func TestProcessCheckout_PropagatesScenarioInContext(t *testing.T) {
	ff := &fakeFraud{resp: &fraudv1.FraudCheckResponse{Recommendation: "APPROVE"}}
	gw := checkout.NewGateway(silentLog(), ff, checkout.FixedOrderCode{Code: "A"})

	ctx := sim.WithScenario(context.Background(), "fraud-declined")
	_, _ = gw.ProcessCheckout(ctx, &checkoutv1.CheckoutRequest{TransactionId: "t4"})
	if ff.lastScenario != "fraud-declined" {
		t.Fatalf("scenario = %q", ff.lastScenario)
	}
}

func TestGRPCServer_Delegates(t *testing.T) {
	ff := &fakeFraud{resp: &fraudv1.FraudCheckResponse{Recommendation: "APPROVE", RiskScore: 5}}
	gw := checkout.NewGateway(silentLog(), ff, checkout.FixedOrderCode{Code: "ORDER-1"})
	srv := &checkout.GRPCServer{Gateway: gw}

	resp, err := srv.ProcessCheckout(context.Background(), &checkoutv1.CheckoutRequest{TransactionId: "g1"})
	if err != nil || resp.GetStatus() != "APPROVED" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
}

func TestNewGateway_Defaults(t *testing.T) {
	ff := &fakeFraud{resp: &fraudv1.FraudCheckResponse{Recommendation: "APPROVE"}}
	gw := checkout.NewGateway(nil, ff, nil)
	resp, err := gw.ProcessCheckout(context.Background(), &checkoutv1.CheckoutRequest{TransactionId: "d1"})
	if err != nil || resp.GetStatus() != "APPROVED" || resp.GetOrderCode() == "" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
}

func TestRandomOrderCode_Generate(t *testing.T) {
	code := checkout.RandomOrderCode{}.Generate()
	if len(code) < 8 {
		t.Fatalf("code too short: %q", code)
	}
}

func TestRandomOrderCode_WithRand(t *testing.T) {
	// Deterministic: seed 1
	a := checkout.RandomOrderCode{Rand: newRandSource(1)}.Generate()
	b := checkout.RandomOrderCode{Rand: newRandSource(1)}.Generate()
	if a != b {
		t.Fatalf("expected deterministic codes, got %q vs %q", a, b)
	}
}
