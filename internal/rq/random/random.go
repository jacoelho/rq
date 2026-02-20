package random

import "math/rand/v2"

var intNFunc = rand.IntN

// IntN returns, as an int, a non-negative pseudo-random number in [0,n).
func IntN(n int) int {
	return intNFunc(n)
}

// SetIntNForTest overrides the random source and returns a restore function.
func SetIntNForTest(fn func(int) int) func() {
	previous := intNFunc
	intNFunc = fn
	return func() {
		intNFunc = previous
	}
}
