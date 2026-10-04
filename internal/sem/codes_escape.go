package sem

// Non-escaping callback parameters.
var (
	cEscapingReturned        = defineCode("E1030", "escaping-returned", "`{name}` is a non-escaping callback and cannot be returned; declare the parameter `escaping fn(...) -> ...`")
	cEscapingStored          = defineCode("E1031", "escaping-stored", "`{name}` is a non-escaping callback and cannot be stored; declare the parameter `escaping fn(...) -> ...`")
	cEscapingCaptured        = defineCode("E1032", "escaping-captured", "`{name}` is a non-escaping callback and cannot be captured by a closure that escapes; declare the parameter `escaping fn(...) -> ...`")
	cEscapingForwarded       = defineCode("E1033", "escaping-forwarded", "`{name}` is a non-escaping callback and cannot be passed to `{to}`, which may keep it; declare the parameter `escaping fn(...) -> ...`")
	cEscapingWitness         = defineCode("E1035", "escaping-witness-mismatch", "`{name}` may keep the callback at parameter {n}, which the interface requires not to escape")
	cEscapingCoerce          = defineCode("E1034", "escaping-coerce-mismatch", "`{actual}` may keep its callback at parameter {n}, but `{expected}` declares it non-escaping; declare the target's parameter `escaping` too, or keep the value in its own type")
	cEscapingForwardedOpaque = defineCode("E1036", "escaping-forwarded-opaque", "`{name}` is a non-escaping callback and cannot be passed to `{to}`, whose parameter type cannot be verified not to keep it; constrain the parameter with `Fn[...]`, `FnMut[...]`, or `FnOnce[...]`, or make the callback `escaping`")
)
