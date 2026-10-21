package sem

var (
	cFieldValueMove    = defineCode("E2153", "field-value-move", "reading `{name}` by value would copy a `{type}`, which is not `Copy`; write `move {name}` to take it out, borrow it with `&{name}`, or clone it")
	cCaptureKeptShared = defineCode("E2154", "capture-kept-and-shared", "`{name}` is kept by a stored closure and also written by a closure passed to a call, which would give one value two owners; write `{name}` outside the closures, or give each closure a local of its own")
)
