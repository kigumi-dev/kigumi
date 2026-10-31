package sem

// Diagnostic catalog, E2110: static exclusivity of `mut self` access.
var (
	cReentrantHeldCapture      = defineCode("E2110", "reentrant-held-capture", "an argument of this call holds a closure that captures `{name}`, which the call exclusively borrows; capture a copy in a separate binding, or end the call before the closure runs")
	cReentrantHiddenCapture    = defineCode("E2111", "reentrant-hidden-capture", "cannot see which closures {what} holds while `{name}` is exclusively borrowed by this call; build the value from literals or `let` bindings in this function, or pass a type that cannot hold a callable")
	cReentrantStoredCapture    = defineCode("E2112", "reentrant-stored-capture", "`{name}` may hold a closure that captures it, but this call exclusively borrows it; end the closure's use before the call, capture a copy, or keep callables out of its fields")
	cCapturedMutBorrowConflict = defineCode("E2113", "captured-mut-borrow-conflict", "`&mut {name}` conflicts with a closure that shares its identity cell; end the closure's use before the borrow, or capture a copy")
)
