package risk

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
)

// VirtualService is the Microcks stand-in: scenario name → fixed response.
type VirtualService struct {
	log   *slog.Logger
	store ScenarioStore
}

// NewVirtualService constructs VirtualService. Nil store → DefaultScenarios().
func NewVirtualService(log *slog.Logger, store ScenarioStore) *VirtualService {
	if log == nil {
		log = slog.Default()
	}
	if store == nil {
		store = DefaultScenarios()
	}
	return &VirtualService{log: log, store: store}
}

// EvaluateRisk serves the scenario response or NotFound.
func (s *VirtualService) EvaluateRisk(ctx context.Context, req *riskv1.RiskRequest) (*riskv1.RiskResponse, error) {
	name := ScenarioNameFromContext(ctx)
	sc, ok := s.store.Lookup(name)
	if !ok {
		s.log.Warn("unknown scenario", "scenario", name, "card", req.GetNftToken())
		return nil, status.Errorf(codes.NotFound, "unknown scenario %q", name)
	}
	s.log.Info("serving scenario",
		"scenario", name,
		"card", req.GetNftToken(),
		"score", sc.Score,
		"decision", sc.Decision,
	)
	return &riskv1.RiskResponse{
		RiskScore:   sc.Score,
		Decision:    sc.Decision,
		RiskFactors: sc.Factors,
	}, nil
}
