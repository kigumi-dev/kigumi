package sem

var (
	cDirectBoxInNoalloc = defineCode("E1570", "direct-box-in-noalloc", "`{name}` is declared `noalloc`, and handing `{type}` to a slot that holds a reference allocates a box for it here; pass it by reference, or drop the declaration")
	cAllocDirectCross   = defineCode("E1610", "alloc-direct-cross", "handing `{type}` to a slot that holds a reference allocates a box for it; keep it in a value of its own, or drop the declaration")
	cAllocCopy          = defineCode("E1700", "alloc-copy", "copying `{type}` here allocates a box of its own for the copy; use the original instead of a copy, or drop the declaration")
)
