package sem

var (
	cDirectBoxInNoalloc = defineCode("E1570", "direct-box-in-noalloc", "`{name}` is declared `noalloc`, and handing `{type}` to a slot that holds a reference allocates a box for it here; pass it by reference, or drop the declaration")
)
