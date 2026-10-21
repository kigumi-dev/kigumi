package sem

var (
	cMethodValueBorrowed  = defineCode("E2163", "method-value-borrowed-receiver", "a method value on move-only `{type}` takes its receiver, but this receiver is borrowed here; call the method directly, or write a lambda")
	cMethodValuePlace     = defineCode("E2164", "method-value-place-receiver", "a method value on move-only `{type}` takes its receiver, which cannot be moved out of {place}; {fix}")
	cMethodValueAsync     = defineCode("E2165", "method-value-async-owned", "an `async` method value cannot take its move-only `{type}` receiver; call the method directly, or write a lambda")
	cMethodValueDropField = defineCode("E2177", "method-value-drop-field", "a method value on move-only `{type}` would move `{name}` out of `{owner}`, which defines a custom `drop`; call the method directly, or add a `take`-style method on `{owner}` that returns the field")
	cMoveCapturedField    = defineCode("E2178", "move-captured-field", "cannot move `{name}` out of `{owner}`, which this closure captured and may run again; move the field into a local before the closure, or capture that local instead")
)
