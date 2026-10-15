package sem

// Diagnostic catalog, E128x: the fallible allocation path and the
// recoverable-only profile.
var (
	cRecoverableOnlyFatalAlloc = defineCode("E1280", "recoverable-only-fatal-alloc", "{what} may terminate on a refused allocation instead of returning it as a value, which the recoverable-only profile forbids; use {fix} instead")
)
