package sem

// Diagnostic catalog, E137x: pay-for-use reachability.
var (
	cStrictCapabilityMissing = defineCode("E1370", "strict-capability-missing", "`{op}` reaches a Target capability this strict profile (freestanding, no default allocator) does not provide; reached through: {path}")
)
