package sem

// Diagnostic catalog, 171x: per-instantiation specialisation.
var (
	cSpecRunaway = defineCode("W1710", "spec-runaway", "`{name}` keeps being instantiated with new type arguments, now `{type}`, so this call stays on the shared body and boxes the values it passes; recurse with the type arguments it already has, or hold the growing value in an interface or an `alloc.Shared`")
	cSpecBudget  = defineCode("W1711", "spec-budget", "calling `{name}` with `{type}` would bring in more than {count} instantiations, so the calls past that stay on their shared bodies and box the values they pass; pass fewer distinct type arguments down the calls, or hold the values in an interface or an `alloc.Shared`")
)
