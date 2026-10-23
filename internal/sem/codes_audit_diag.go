package sem

// Diagnostic catalog, E2239-E2241: variant type arguments and comptime failures.
var (
	cComptimePanic   = defineCode("E2240", "comptime-panic", "the `comptime` block panicked: {reason}; a panic stops compilation, so fix the failing step or compute the value at run time")
	cComptimeLimit   = defineCode("E2241", "comptime-limit", "the `comptime` block exceeded its {limit}; do less work in the block or compute the value at run time (a `comptime` block allows 3000 nested calls, a run 100000)")
	cVariantTypeArgs = defineCode("E2239", "variant-type-args", "`{name}` is a variant and takes no type arguments of its own; write `{type}.{name}(...)`, or annotate the binding and call `{name}(...)`")
)
