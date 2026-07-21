package fraud

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/servicemesh/reference-app/internal/sim"
)

// ModeLocal routes to Microcks when a simulation header is present.
const ModeLocal = "local"

// ModeMesh always uses the external endpoint (Istio handles virtualization).
const ModeMesh = "mesh"

// LocalRiskResolver implements SIMULATION_MODE=local dial selection.
//
//	local + scenario header → microcks
//	otherwise               → external
type LocalRiskResolver struct {
	Log      *slog.Logger
	Mode     string // "local" | "mesh" (or empty → mesh)
	External RiskEvaluator
	Microcks RiskEvaluator
	// Labels used only for logging / assertions.
	ExternalTarget string
	MicrocksTarget string
}

// Resolve picks the evaluator for this request context.
func (r *LocalRiskResolver) Resolve(ctx context.Context) (ResolvedRisk, error) {
	log := r.Log
	if log == nil {
		log = slog.Default()
	}
	scenario := sim.ScenarioFromContext(ctx)
	useMicrocks := r.Mode == ModeLocal && scenario != "" && r.Microcks != nil

	if useMicrocks {
		log.Info("simulation header present — routing to Microcks",
			"scenario", scenario, "endpoint", r.MicrocksTarget)
		return ResolvedRisk{Client: r.Microcks, Target: r.MicrocksTarget}, nil
	}

	log.Info("calling real external risk API", "endpoint", r.ExternalTarget)
	if r.External == nil {
		return ResolvedRisk{}, fmt.Errorf("external risk evaluator not configured")
	}
	return ResolvedRisk{Client: r.External, Target: r.ExternalTarget}, nil
}

// FixedRiskResolver always returns the same evaluator (mesh mode / simple tests).
type FixedRiskResolver struct {
	Client RiskEvaluator
	Target string
}

// Resolve returns the fixed client.
func (f FixedRiskResolver) Resolve(ctx context.Context) (ResolvedRisk, error) {
	if f.Client == nil {
		return ResolvedRisk{}, fmt.Errorf("risk evaluator not configured")
	}
	target := f.Target
	if target == "" {
		target = "fixed"
	}
	return ResolvedRisk{Client: f.Client, Target: target}, nil
}
