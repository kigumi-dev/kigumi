package sem

// defer / defer(fail): a binding a cleanup consumes is reserved until the scope exits.
var (
	cCleanupReservedAssign = defineCode("E2100", "cleanup-reserved-assign", "`{name}` is reserved by a cleanup that consumes it and cannot be assigned before the scope exits; bind the new value to another name, or let the cleanup borrow the binding instead of consuming it")
	cCleanupReservedMove   = defineCode("E2101", "cleanup-reserved-move", "`{name}` is reserved by a cleanup that consumes it and cannot be moved before the scope exits; move it after the scope, return it as the success value when only a `defer(fail)` reserves it, or drop the cleanup and consume it yourself")
	cCleanupReservedTwice  = defineCode("E2102", "cleanup-reserved-twice", "`{name}` is already consumed by an earlier cleanup; only one cleanup in scope may consume a binding")
	cCleanupReadsMoved     = defineCode("E2103", "cleanup-reads-moved", "a cleanup reads `{name}`, which can already be moved when the cleanup runs; keep the binding in place until the scope exits, or save what the cleanup needs in another binding")
)
