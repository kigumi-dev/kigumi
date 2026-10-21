package sem

var (
	cErrorParamBox       = defineCode("E2230", "error-param-box", "`{type}` is a type parameter bounded by `Error`, so its concrete type is not known here and it cannot become a dynamic `Error`; keep the type parameter in the signature, or convert the value where the type is concrete")
	cComptimeConstUse    = defineCode("E2231", "comptime-const-use", "`{name}` is initialized by `comptime`, so its value is not known where this needs it; read it inside a function body instead")
	cComptimeInitUntyped = defineCode("E2232", "comptime-init-untyped", "a `comptime` initializer needs a type annotation and must be the whole initializer, as in `const c: Int = comptime f()`; write `comptime (a + 1)` or `comptime { a + 1 }` for an expression")
)
