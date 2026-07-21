package fraud_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
	"github.com/servicemesh/reference-app/internal/fraud"
	"github.com/servicemesh/reference-app/internal/sim"
)

type fakeRisk struct {
	resp *riskv1.RiskResponse
	err  error
}

func (f *fakeRisk) EvaluateRisk(ctx context.Context, req *riskv1.RiskRequest) (*riskv1.RiskResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

type fakeResolver struct {
	resolved fraud.ResolvedRisk
	err      error
}

func (f fakeResolver) Resolve(ctx context.Context) (fraud.ResolvedRisk, error) {
	return f.resolved, f.err
}

func silent() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCheckFraud_ApproveLowScore(t *testing.T) {
	c := fraud.NewChecker(silent(), fakeResolver{
		resolved: fraud.ResolvedRisk{
			Client: &fakeRisk{resp: &riskv1.RiskResponse{RiskScore: 10, Decision: "APPROVE"}},
			Target: "external",
		},
	})
	resp, err := c.CheckFraud(context.Background(), &fraudv1.FraudCheckRequest{TransactionId: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Recommendation != "APPROVE" || resp.RiskScore != 10 {
		t.Fatalf("%+v", resp)
	}
}

func TestCheckFraud_DeclineHighScore(t *testing.T) {
	c := fraud.NewChecker(silent(), fakeResolver{
		resolved: fraud.ResolvedRisk{
			Client: &fakeRisk{resp: &riskv1.RiskResponse{RiskScore: 92, Decision: "DECLINE"}},
			Target: "microcks",
		},
	})
	resp, err := c.CheckFraud(context.Background(), &fraudv1.FraudCheckRequest{TransactionId: "t2"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Recommendation != "DECLINE" || resp.Reason != "HIGH_RISK_SCORE" {
		t.Fatalf("%+v", resp)
	}
}

func TestCheckFraud_DeclineByDecisionOnly(t *testing.T) {
	// Score below threshold but explicit DECLINE decision.
	c := fraud.NewChecker(silent(), fakeResolver{
		resolved: fraud.ResolvedRisk{
			Client: &fakeRisk{resp: &riskv1.RiskResponse{RiskScore: 10, Decision: "DECLINE"}},
			Target: "x",
		},
	})
	resp, _ := c.CheckFraud(context.Background(), &fraudv1.FraudCheckRequest{TransactionId: "t3"})
	if resp.Recommendation != "DECLINE" {
		t.Fatalf("%+v", resp)
	}
}

func TestCheckFraud_ResolveError(t *testing.T) {
	c := fraud.NewChecker(silent(), fakeResolver{err: errors.New("no dial")})
	resp, err := c.CheckFraud(context.Background(), &fraudv1.FraudCheckRequest{TransactionId: "t4"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Reason != "RISK_SERVICE_UNAVAILABLE" || resp.Recommendation != "DECLINE" {
		t.Fatalf("%+v", resp)
	}
}

func TestCheckFraud_EvaluateError(t *testing.T) {
	c := fraud.NewChecker(silent(), fakeResolver{
		resolved: fraud.ResolvedRisk{
			Client: &fakeRisk{err: errors.New("rpc failed")},
			Target: "microcks",
		},
	})
	resp, err := c.CheckFraud(context.Background(), &fraudv1.FraudCheckRequest{TransactionId: "t5"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Reason != "RISK_SERVICE_ERROR" {
		t.Fatalf("%+v", resp)
	}
}

func TestMapRiskToRecommendation_Threshold(t *testing.T) {
	rec, reason := fraud.MapRiskToRecommendation(&riskv1.RiskResponse{RiskScore: 70})
	if rec != "DECLINE" || reason != "HIGH_RISK_SCORE" {
		t.Fatalf("%s %s", rec, reason)
	}
	rec, reason = fraud.MapRiskToRecommendation(&riskv1.RiskResponse{RiskScore: 69})
	if rec != "APPROVE" || reason != "" {
		t.Fatalf("%s %s", rec, reason)
	}
}

func TestGRPCServer_Delegates(t *testing.T) {
	c := fraud.NewChecker(nil, fakeResolver{
		resolved: fraud.ResolvedRisk{
			Client: &fakeRisk{resp: &riskv1.RiskResponse{RiskScore: 5, Decision: "APPROVE"}},
			Target: "t",
		},
	})
	srv := &fraud.GRPCServer{Checker: c}
	resp, err := srv.CheckFraud(context.Background(), &fraudv1.FraudCheckRequest{TransactionId: "g"})
	if err != nil || resp.GetRecommendation() != "APPROVE" {
		t.Fatalf("%+v %v", resp, err)
	}
}

func TestLocalRiskResolver_LocalWithScenarioUsesMicrocks(t *testing.T) {
	ext := &fakeRisk{resp: &riskv1.RiskResponse{RiskScore: 1}}
	mc := &fakeRisk{resp: &riskv1.RiskResponse{RiskScore: 99}}
	r := &fraud.LocalRiskResolver{
		Log:            silent(),
		Mode:           fraud.ModeLocal,
		External:       ext,
		Microcks:       mc,
		ExternalTarget: "ext",
		MicrocksTarget: "mc",
	}
	ctx := sim.WithScenario(context.Background(), "fraud-declined")
	got, err := r.Resolve(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != "mc" || got.Client != mc {
		t.Fatalf("%+v", got)
	}
}

func TestLocalRiskResolver_LocalWithoutScenarioUsesExternal(t *testing.T) {
	ext := &fakeRisk{}
	mc := &fakeRisk{}
	r := &fraud.LocalRiskResolver{
		Mode: fraud.ModeLocal, External: ext, Microcks: mc,
		ExternalTarget: "ext", MicrocksTarget: "mc",
	}
	got, err := r.Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != "ext" {
		t.Fatalf("%+v", got)
	}
}

func TestLocalRiskResolver_MeshIgnoresScenario(t *testing.T) {
	ext := &fakeRisk{}
	mc := &fakeRisk{}
	r := &fraud.LocalRiskResolver{
		Mode: fraud.ModeMesh, External: ext, Microcks: mc,
		ExternalTarget: "ext", MicrocksTarget: "mc",
	}
	ctx := sim.WithScenario(context.Background(), "fraud-declined")
	got, err := r.Resolve(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != "ext" {
		t.Fatalf("mesh must not dial microcks: %+v", got)
	}
}

func TestLocalRiskResolver_MissingExternal(t *testing.T) {
	r := &fraud.LocalRiskResolver{Mode: fraud.ModeMesh}
	_, err := r.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLocalRiskResolver_LocalScenarioWithoutMicrocksFallsBack(t *testing.T) {
	// Header present but Microcks not wired → use external (useMicrocks requires Microcks != nil).
	ext := &fakeRisk{}
	r := &fraud.LocalRiskResolver{
		Mode: fraud.ModeLocal, External: ext, Microcks: nil,
		ExternalTarget: "ext",
	}
	ctx := sim.WithScenario(context.Background(), "fraud-declined")
	got, err := r.Resolve(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != "ext" {
		t.Fatalf("%+v", got)
	}
}

func TestLocalRiskResolver_NilLog(t *testing.T) {
	r := &fraud.LocalRiskResolver{
		Mode: fraud.ModeMesh, External: &fakeRisk{}, ExternalTarget: "e",
	}
	if _, err := r.Resolve(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestFixedRiskResolver(t *testing.T) {
	c := &fakeRisk{}
	r := fraud.FixedRiskResolver{Client: c, Target: "fixed-t"}
	got, err := r.Resolve(context.Background())
	if err != nil || got.Client != c || got.Target != "fixed-t" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestFixedRiskResolver_Defaults(t *testing.T) {
	_, err := fraud.FixedRiskResolver{}.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	got, err := fraud.FixedRiskResolver{Client: &fakeRisk{}}.Resolve(context.Background())
	if err != nil || got.Target != "fixed" {
		t.Fatalf("%+v %v", got, err)
	}
}
