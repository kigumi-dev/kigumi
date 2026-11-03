//go:build !race

package borrow

// raceDetector is false in a plain build: the CPU budgets in
// TestParamStoreSitesComponentCost compare analysis cost without the race
// detector's shadow-memory overhead.
const raceDetector = false
