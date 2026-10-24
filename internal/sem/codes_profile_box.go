package sem

var (
	cProfileBoxRecoverable = defineCode("E2313", "box-in-recoverable-only", "`{name}` passed the recoverable-only profile as if `{type}` stayed unboxed, yet handing it to a slot that holds a reference allocates a box here that a refused allocation would end the program on; call a form that takes it unboxed, or allocate inside an `allocator` block")
	cProfileBoxNoDefault   = defineCode("E2314", "box-without-default-allocator", "`{name}` passed --allocator none as if `{type}` stayed unboxed, yet handing it to a slot that holds a reference allocates a box from the default allocator here; call a form that takes it unboxed, or allocate inside an `allocator` block")
)
