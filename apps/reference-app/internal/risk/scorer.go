package risk

import "strings"

// TokenPrefixScorer implements CardScorer with the POC token rules:
//
//	tok_high_risk_* → 85 DECLINE
//	tok_low_risk_*  → 10 APPROVE
//	else            → 20 APPROVE
type TokenPrefixScorer struct{}

// Score implements CardScorer.
func (TokenPrefixScorer) Score(cardToken string) ScoreResult {
	switch {
	case strings.HasPrefix(cardToken, "tok_high_risk_"):
		return ScoreResult{Score: 85, Decision: "DECLINE", Factors: []string{"HIGH_RISK_TOKEN"}}
	case strings.HasPrefix(cardToken, "tok_low_risk_"):
		return ScoreResult{Score: 10, Decision: "APPROVE"}
	default:
		return ScoreResult{Score: 20, Decision: "APPROVE"}
	}
}
