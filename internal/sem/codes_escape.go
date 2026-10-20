package sem

// Non-escaping callback parameters.
var (
	cEscapingReturned        = defineCode("E1030", "escaping-returned", "`{name}` is a non-escaping callback and cannot be returned; declare the parameter {form}")
	cEscapingStored          = defineCode("E1031", "escaping-stored", "`{name}` is a non-escaping callback and cannot be stored; declare the parameter {form}")
	cEscapingCaptured        = defineCode("E1032", "escaping-captured", "`{name}` is a non-escaping callback and cannot be captured by a closure that escapes; declare the parameter {form}")
	cEscapingForwarded       = defineCode("E1033", "escaping-forwarded", "`{name}` is a non-escaping callback and cannot be passed to `{to}`, which may keep it; declare the parameter {form}")
	cEscapingWitness         = defineCode("E1035", "escaping-witness-mismatch", "`{name}` may keep the callback at parameter {n}, which the interface requires not to escape")
	cEscapingCoerce          = defineCode("E1034", "escaping-coerce-mismatch", "`{actual}` may keep its callback at parameter {n}, but `{expected}` declares it non-escaping; declare the target's parameter `escaping` too, or keep the value in its own type")
	cEscapingForwardedOpaque = defineCode("E1036", "escaping-forwarded-opaque", "`{name}` is a non-escaping callback and cannot be passed to `{to}`, whose parameter type cannot be verified not to keep it; constrain the parameter with `Fn[...]`, `FnMut[...]`, or `FnOnce[...]`, or make the callback `escaping`")
	cEscapingNested          = defineCode("E2168", "escaping-nested-mismatch", "`{actual}` differs from `{expected}` only in which callback parameters may be kept, and a value cannot move between them inside another type; write the escaping marks of the target to match, or keep the value in its own type")
	cOnceCalledAgain         = defineCode("E2175", "fnonce-called-again", "`{name}` is bound by `FnOnce` and may already have been called, so it cannot be called again; bind it with `FnMut` or `Fn` to call it more than once")
	cOnceCalledInLoop        = defineCode("E2176", "fnonce-called-in-loop", "`{name}` is bound by `FnOnce` and cannot be called inside a loop or a closure, which may call it more than once; bind it with `FnMut` or `Fn`, or call it outside the loop or closure")
	cEscapingNotCallable     = defineCode("E2166", "escaping-not-callable", "`escaping` applies to a fn type or a type parameter bounded by `Fn`, `FnMut` or `FnOnce`, not `{type}`; remove `escaping`")
	cEscapingUnboundedParam  = defineCode("E2167", "escaping-unbounded-param", "`escaping` on type parameter `{name}`, which is not bounded by `Fn`, `FnMut` or `FnOnce`; add the bound or remove `escaping`")
)
