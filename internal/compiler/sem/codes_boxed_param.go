package sem

var (
	cAllocBoxedArg = defineCode("E2147", "alloc-boxed-param-arg", "this computed `{type}` goes to a parameter its callee keeps in a box, so the call allocates one for it; pass a constant or a value that is already boxed, or drop the declaration")
)
