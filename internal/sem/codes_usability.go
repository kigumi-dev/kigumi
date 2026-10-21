package sem

var (
	cUseAfterMove   = defineCode("E2197", "use-after-move", "`{name}` was moved, so it cannot be used again")
	cBorrowConflict = defineCode("E2198", "borrow-conflict", "`{name}` is borrowed and this use conflicts with the borrow; end the borrow first or use a copy")
	cSyntaxError    = defineCode("E2199", "syntax-error", "syntax error: the message names what the parser expected")
)
