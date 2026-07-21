package payment_test

import "math/rand"

func newRandSource(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}
