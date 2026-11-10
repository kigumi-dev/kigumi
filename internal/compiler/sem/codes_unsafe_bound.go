package sem

// Diagnostic catalog, E2221-E2223: memory-safety gates outside the borrow checker.
var cUnsafeFnBound = defineCode("E2221", "unsafe-fn-bound", "`{name}` is {kind} and cannot satisfy `{bound}`; wrap it in a lambda inside `unsafe { }` and pass the lambda")

var cLeasePassed = defineCode("E2223", "lease-pointer-passed", "`{fn}`'s callback passes its leased pointer to `{callee}`, whose body may keep it past the call; keep the pointer inside the callback, or use a retained carrier (`ffi.Pin`) for data that must outlive this call")

var cLeaseStored = defineCode("E2222", "lease-pointer-stored", "`{fn}`'s callback stores its leased pointer in `{name}`, which outlives the call; keep the pointer inside the callback, or use a retained carrier (`ffi.Pin`) for data that must outlive this call")
