package checkout

import (
	"fmt"
	"math/rand"
)

// RandomOrderCode is the production OrderCodeGenerator.
type RandomOrderCode struct {
	// Rand is optional; nil uses the global math/rand source.
	Rand *rand.Rand
}

// Generate returns ORDER-NNNN-L.
func (g RandomOrderCode) Generate() string {
	r := g.Rand
	if r == nil {
		//nolint:gosec // non-crypto auth code for demo
		return fmt.Sprintf("ORDER-%04d-%c", rand.Intn(10000), 'A'+rune(rand.Intn(26)))
	}
	return fmt.Sprintf("ORDER-%04d-%c", r.Intn(10000), 'A'+rune(r.Intn(26)))
}

// FixedOrderCode always returns the same code (tests).
type FixedOrderCode struct {
	Code string
}

// Generate returns Code.
func (f FixedOrderCode) Generate() string { return f.Code }
