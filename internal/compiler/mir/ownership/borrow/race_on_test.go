//go:build race

package borrow

// raceDetector is true under -race: the shadow-memory checks inflate the CPU
// time past the plain-build budgets, so those comparisons only run without it.
const raceDetector = true
