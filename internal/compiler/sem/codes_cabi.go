package sem

var (
	cCSymbolConflict  = defineCode("E2218", "c-symbol-conflict", "the C symbol `{name}` {why}; declare each C symbol once, with one signature, and export only names the C library does not define")
	cCVariadicNotLast = defineCode("E2219", "c-variadic-not-last", "a C `...` must be the last parameter of an extern declaration; move it to the end")
	cCBitPattern      = defineCode("E2220", "c-invalid-bit-pattern", "`{type}` cannot be an extern parameter, because C can store bit patterns that are not a valid {what}; use the integer type `{raw}` and convert it in Kigumi")
)
