package sem

var (
	cDirectBoxInNoalloc = defineCode("E1570", "direct-box-in-noalloc", "`{name}` is declared `noalloc`, and handing `{type}` to a slot that holds a reference allocates a box for it here; pass it by reference, or drop the declaration")
	cAllocDirectCross   = defineCode("E1610", "alloc-direct-cross", "handing `{type}` to a slot that holds a reference allocates a box for it; keep it in a value of its own, or drop the declaration")
)
