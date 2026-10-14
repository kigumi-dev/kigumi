package sem

var (
	cDirectBoxInNoalloc   = defineCode("E1570", "direct-box-in-noalloc", "`{name}` is declared `noalloc`, and handing `{type}` to a slot that holds a reference allocates a box for it here; pass it by reference, or drop the declaration")
	cInferredBoxInNoalloc = defineCode("E1770", "inferred-box-in-noalloc", "`{name}` is inferred `noalloc`, yet handing `{type}` to a slot that holds a reference allocates a box for it here; the summary callers rely on is wrong, so report this")
	cAllocDirectCross     = defineCode("E1610", "alloc-direct-cross", "handing `{type}` to a slot that holds a reference allocates a box for it; keep it in a value of its own, or drop the declaration")
	cAllocFnValueResult   = defineCode("E1840", "alloc-fn-value-result", "`{name}` returns `{type}` unboxed, so every call through this function value allocates a box for the result; call `{name}` directly, or drop the declaration")
	cAllocCallResultBox   = defineCode("E1841", "alloc-call-result-box", "this call goes through a function value or an interface, so the `{type}` it returns arrives in a box the callee allocates; call the function directly, or drop the declaration")
	cResultBoxInNoalloc   = defineCode("E1842", "result-box-in-noalloc", "`{name}` is declared `noalloc`, and the `{type}` this call returns through a function value or an interface arrives in a box allocated for it; call the function directly, or drop the declaration")
	cInferredResultBox    = defineCode("E1843", "inferred-result-box-in-noalloc", "`{name}` is inferred `noalloc`, yet the `{type}` this call returns through a function value or an interface arrives in a box allocated for it; the summary callers rely on is wrong, so report this")
	cAllocCopy            = defineCode("E1700", "alloc-copy", "copying `{type}` here allocates a box of its own for the copy; use the original instead of a copy, or drop the declaration")
)
