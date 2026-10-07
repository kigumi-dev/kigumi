package sem

// Diagnostic catalog, E137x: pay-for-use reachability.
var (
	cCapabilityNotProvided = defineCode("E1370", "capability-not-provided", "`{op}` reaches the `{capability}` capability, which this target does not provide: {reason}; reached through: {path}")
)
