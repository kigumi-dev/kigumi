package sem

// Diagnostic catalog, E2257-E2259: context keys at drops, foreign entries and writes.
var (
	cDropHiddenArg   = defineCode("E2257", "drop-hidden-arg", "`{name}` uses {what}, but a drop runs wherever its value is released and receives no hidden argument there; read what the drop needs when the value is made and keep it in a field")
	cCtxForeignEntry = defineCode("E2258", "ctx-foreign-entry", "{what} requires context key `{key}`, but it is {where}, where no provider is bound; supply the key with `context (...) { }` inside it")
	cCtxKeyAssign    = defineCode("E2259", "ctx-key-assign", "context key `{key}` cannot be {what}; {why}")
)

const (
	ctxKeyWholeWhy = "a key is a lease on its provider's field, not a variable; supply a new value with a nested `context (...) { }`"
	ctxKeyCopyWhy  = "its type is Copy, so a function that reads the key gets a copy and the write would be lost; supply a new value with a nested `context (...) { }`, or give the key a non-Copy type"

	ctxComptimeWhat  = "this `comptime` block"
	ctxComptimeWhere = "evaluated at compile time"
)
