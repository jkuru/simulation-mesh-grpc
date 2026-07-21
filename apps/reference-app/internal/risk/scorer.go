package risk

import "strings"

// TokenPrefixScorer implements NftScorer with the POC token rules:
//
//	nft_high_risk_* → 85 DECLINE
//	nft_low_risk_*  → 10 APPROVE
//	else            → 20 APPROVE
type TokenPrefixScorer struct{}

// Score implements NftScorer.
func (TokenPrefixScorer) Score(nftToken string) ScoreResult {
	switch {
	case strings.HasPrefix(nftToken, "nft_high_risk_"):
		return ScoreResult{Score: 85, Decision: "DECLINE", Factors: []string{"HIGH_RISK_TOKEN"}}
	case strings.HasPrefix(nftToken, "nft_low_risk_"):
		return ScoreResult{Score: 10, Decision: "APPROVE"}
	default:
		return ScoreResult{Score: 20, Decision: "APPROVE"}
	}
}
