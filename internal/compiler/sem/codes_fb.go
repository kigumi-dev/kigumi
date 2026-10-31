package sem

// Diagnostic catalog, E2212 to E2214: errors found while repairing internal failures.
var (
	cTypeNomono    = defineCode("E2213", "type-nomono-recursion", "`{from}` requires `{to}`{via}, so the type expands without end; use the parameter unchanged in the recursive field")
	cConstTooLarge = defineCode("E2214", "const-too-large", "constant integer arithmetic is limited to {bits} bits, and a shift count cannot be negative")
	cComptimeFlow  = defineCode("E2212", "comptime-flow", "{what} cannot leave a `comptime` block, which has no enclosing function to return to or suspend; handle the value inside the block")
)
