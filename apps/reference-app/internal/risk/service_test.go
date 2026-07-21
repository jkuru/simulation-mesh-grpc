package risk_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
	"github.com/servicemesh/reference-app/internal/risk"
	"github.com/servicemesh/reference-app/internal/sim"
)

func silent() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestTokenPrefixScorer(t *testing.T) {
	s := risk.TokenPrefixScorer{}
	cases := []struct {
		token string
		score int32
		dec   string
	}{
		{"nft_low_risk_4242", 10, "APPROVE"},
		{"nft_high_risk_9999", 85, "DECLINE"},
		{"nft_other", 20, "APPROVE"},
	}
	for _, tc := range cases {
		got := s.Score(tc.token)
		if got.Score != tc.score || got.Decision != tc.dec {
			t.Fatalf("%s: %+v", tc.token, got)
		}
	}
	hi := s.Score("nft_high_risk_x")
	if len(hi.Factors) != 1 || hi.Factors[0] != "HIGH_RISK_TOKEN" {
		t.Fatalf("factors: %+v", hi.Factors)
	}
}

func TestExternalService_EvaluateRisk(t *testing.T) {
	svc := risk.NewExternalService(silent(), nil)
	resp, err := svc.EvaluateRisk(context.Background(), &riskv1.RiskRequest{
		NftToken: "nft_low_risk_1", AmountCents: 50,
	})
	if err != nil || resp.RiskScore != 10 || resp.Decision != "APPROVE" {
		t.Fatalf("%+v %v", resp, err)
	}
}

func TestVirtualService_KnownScenario(t *testing.T) {
	svc := risk.NewVirtualService(silent(), nil)
	ctx := sim.WithScenario(context.Background(), risk.ScenarioFraudDeclined)
	resp, err := svc.EvaluateRisk(ctx, &riskv1.RiskRequest{NftToken: "nft_low_risk_1"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.RiskScore != 92 || resp.Decision != "DECLINE" {
		t.Fatalf("%+v", resp)
	}
	if len(resp.RiskFactors) != 2 {
		t.Fatalf("factors: %v", resp.RiskFactors)
	}
}

func TestVirtualService_ApprovedScenario(t *testing.T) {
	svc := risk.NewVirtualService(silent(), risk.DefaultScenarios())
	ctx := sim.WithScenario(context.Background(), risk.ScenarioFraudApproved)
	resp, err := svc.EvaluateRisk(ctx, &riskv1.RiskRequest{})
	if err != nil || resp.RiskScore != 5 {
		t.Fatalf("%+v %v", resp, err)
	}
}

func TestVirtualService_UnknownScenario(t *testing.T) {
	svc := risk.NewVirtualService(silent(), risk.DefaultScenarios())
	ctx := sim.WithScenario(context.Background(), "no-such")
	_, err := svc.EvaluateRisk(ctx, &riskv1.RiskRequest{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestScenarioNameFromContext_Sources(t *testing.T) {
	if risk.ScenarioNameFromContext(context.Background()) != "" {
		t.Fatal("empty")
	}
	ctx := sim.WithScenario(context.Background(), "from-sim")
	if risk.ScenarioNameFromContext(ctx) != "from-sim" {
		t.Fatal("sim")
	}

	md := metadata.Pairs(risk.MicrocksOperationHeader, "from-microcks")
	ctx = metadata.NewIncomingContext(context.Background(), md)
	if risk.ScenarioNameFromContext(ctx) != "from-microcks" {
		t.Fatal("microcks header")
	}

	md = metadata.Pairs(sim.Header, "from-raw")
	ctx = metadata.NewIncomingContext(context.Background(), md)
	if risk.ScenarioNameFromContext(ctx) != "from-raw" {
		t.Fatal("raw header")
	}
}

func TestMapScenarioStore_Lookup(t *testing.T) {
	m := risk.DefaultScenarios()
	sc, ok := m.Lookup(risk.ScenarioFraudApproved)
	if !ok || sc.Score != 5 {
		t.Fatalf("%+v %v", sc, ok)
	}
	_, ok = m.Lookup("missing")
	if ok {
		t.Fatal("expected miss")
	}
}

func TestGRPCServer_Delegates(t *testing.T) {
	svc := risk.NewExternalService(nil, risk.TokenPrefixScorer{})
	srv := &risk.GRPCServer{Handler: svc}
	resp, err := srv.EvaluateRisk(context.Background(), &riskv1.RiskRequest{NftToken: "nft_high_risk_1"})
	if err != nil || resp.GetRiskScore() != 85 {
		t.Fatalf("%+v %v", resp, err)
	}
}

func TestNewVirtualService_NilDefaults(t *testing.T) {
	svc := risk.NewVirtualService(nil, nil)
	ctx := sim.WithScenario(context.Background(), risk.ScenarioFraudApproved)
	resp, err := svc.EvaluateRisk(ctx, &riskv1.RiskRequest{})
	if err != nil || resp.GetRiskScore() != 5 {
		t.Fatalf("%+v %v", resp, err)
	}
}

func TestNewExternalService_NilDefaults(t *testing.T) {
	svc := risk.NewExternalService(nil, nil)
	resp, err := svc.EvaluateRisk(context.Background(), &riskv1.RiskRequest{NftToken: "nft_low_risk_x"})
	if err != nil || resp.GetRiskScore() != 10 {
		t.Fatalf("%+v %v", resp, err)
	}
}
