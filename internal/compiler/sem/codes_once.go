package sem

var (
	cClosureCalledOnce = defineCode("E2151", "closure-called-once", "`{name}` consumes `{place}` when called and can be called only once; call it a single time, or have the closure borrow `{place}` instead of consuming it")
	cOnceViaBorrow     = defineCode("E2180", "once-called-through-borrow", "`{name}` can be called only once but is reached through a borrow, so every call of this function would call it again; take the value by move (`move self` or a by-value parameter) before calling it")
	cFnOnceCalledOnce  = defineCode("E2152", "fn-once-called-once", "`{name}` is a `FnOnce` value and can be called only once; call it a single time, or bound its type with `Fn` or `FnMut` instead")
)
