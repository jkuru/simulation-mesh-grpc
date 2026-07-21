// Package risk implements third-party and virtual RiskService backends.
//
// Interface-first: NftScorer and ScenarioStore are the ports; ExternalService
// and VirtualService are the use-cases bound to those ports.
package risk

// ScoreResult is a pure domain outcome from NFT risk scoring.
type ScoreResult struct {
	Score   int32
	Decision string
	Factors []string
}

// NftScorer scores a NFT token for the "real" external risk path.
type NftScorer interface {
	Score(nftToken string) ScoreResult
}

// Scenario is a named virtual response.
type Scenario struct {
	Score    int32
	Decision string
	Factors  []string
}

// ScenarioStore looks up virtual responses by scenario name.
type ScenarioStore interface {
	// Lookup returns the scenario or false if unknown.
	Lookup(name string) (Scenario, bool)
}
