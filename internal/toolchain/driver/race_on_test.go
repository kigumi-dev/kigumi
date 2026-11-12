//go:build race

package driver_test

// raceDetector is true under -race: shadow memory inflates the process RSS and
// CPU time, so the resource caps only compare plain builds.
const raceDetector = true
