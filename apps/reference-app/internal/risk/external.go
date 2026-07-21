package risk

import (
	"context"
	"log/slog"

	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
	"github.com/servicemesh/reference-app/internal/sim"
)

// ExternalService is the "real" third-party RiskService stand-in.
type ExternalService struct {
	log    *slog.Logger
	scorer NftScorer
}

// NewExternalService constructs ExternalService. Nil scorer → TokenPrefixScorer.
func NewExternalService(log *slog.Logger, scorer NftScorer) *ExternalService {
	if log == nil {
		log = slog.Default()
	}
	if scorer == nil {
		scorer = TokenPrefixScorer{}
	}
	return &ExternalService{log: log, scorer: scorer}
}

// EvaluateRisk scores the NFT token.
func (s *ExternalService) EvaluateRisk(ctx context.Context, req *riskv1.RiskRequest) (*riskv1.RiskResponse, error) {
	result := s.scorer.Score(req.GetNftToken())
	s.log.Info("EvaluateRisk",
		"nft", req.GetNftToken(),
		"amount_cents", req.GetAmountCents(),
		"score", result.Score,
		"decision", result.Decision,
		"simulation", sim.ScenarioFromContext(ctx),
	)
	return &riskv1.RiskResponse{
		RiskScore:   result.Score,
		Decision:    result.Decision,
		RiskFactors: result.Factors,
	}, nil
}
