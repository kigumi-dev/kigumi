package sem

// Diagnostic catalog, E2260-E2262 and E2270: receiver replacement, scalar copies, lent views and copied receivers.
var cAsyncSelfReplace = defineCode("E2260", "async-self-replace", "`{name}` lends `self` to {callee}, which may replace it as a whole, in an `async` body, where the caller's storage is out of reach by the time it runs; assign self's fields instead, or make the method synchronous")

var cScalarCopyResult = defineCode("E2261", "scalar-copy-result", "`{name}` holds a copy of the scalar from a `&mut` match, so returning or storing it as `&mut` aliases nothing the caller lent; use the scalar by value, or a `&mut` of the whole place")

var cLentViewSpawned = defineCode("E2262", "lent-view-spawned", "a future holding an `&Interface` view made in this frame cannot be spawned or passed to a call that may spawn it, since the frame ends before the task may next run; pass `&T` of the concrete type, or await the future here")

var cMethodValueCopiedRecv = defineCode("E2270", "method-value-copied-recv", "`{name}` takes `mut self`, but a method value runs it on its own copy of the receiver, so its writes are lost; call the method directly, or write a lambda that calls it")
