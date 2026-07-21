package payment

import (
	"fmt"
	"math/rand"
)

// RandomAuthCode is the production AuthCodeGenerator.
type RandomAuthCode struct {
	// Rand is optional; nil uses the global math/rand source.
	Rand *rand.Rand
}

// Generate returns AUTH-NNNN-L.
func (g RandomAuthCode) Generate() string {
	r := g.Rand
	if r == nil {
		//nolint:gosec // non-crypto auth code for demo
		return fmt.Sprintf("AUTH-%04d-%c", rand.Intn(10000), 'A'+rune(rand.Intn(26)))
	}
	return fmt.Sprintf("AUTH-%04d-%c", r.Intn(10000), 'A'+rune(r.Intn(26)))
}

// FixedAuthCode always returns the same code (tests).
type FixedAuthCode struct {
	Code string
}

// Generate returns Code.
func (f FixedAuthCode) Generate() string { return f.Code }
