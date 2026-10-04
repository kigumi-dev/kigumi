package sem

// Diagnostic catalog, E108x: export(interrupt) handler safety.
var (
	cInterruptAlloc        = defineCode("E1080", "interrupt-handler-alloc", "export(interrupt) handler `{name}` may allocate or deallocate memory; interrupt handlers must never touch the allocator")
	cInterruptPanic        = defineCode("E1081", "interrupt-handler-panic", "export(interrupt) handler `{name}` may panic; interrupt handlers must never unwind")
	cInterruptUnsafeExtern = defineCode("E1082", "interrupt-handler-unsafe-extern", "export(interrupt) handler `{name}` may block or reach unverified code; mark the callee `noalloc` or give it a body the compiler can check")
	cInterruptUnknown      = defineCode("E1083", "interrupt-handler-unknown", "export(interrupt) handler `{name}` reaches a call whose target is not known until run time; interrupt handlers may only call statically resolvable functions")
	cInterruptArgs         = defineCode("E1084", "interrupt-handler-args", "export(interrupt) handler `{name}` takes parameters; the generated wrapper cannot box them without allocating; poll `host.signals()` or a static flag from the handler instead")
	cInterruptReturn       = defineCode("E1085", "interrupt-handler-return", "export(interrupt) handler `{name}` must return `Unit`; the generated wrapper cannot box a return value without allocating")
	cInterruptExtern       = defineCode("E1086", "interrupt-extern", "`interrupt` belongs to `export` blocks; extern declarations have no body to check")
	cInterruptDrop         = defineCode("E1087", "interrupt-handler-drop", "export(interrupt) handler `{name}` drops a value of type `{type}` in the handler; keep heap-owning values out of interrupt handlers")
)
