package sem

// Diagnostic catalog, E108x: export(interrupt) handler safety.
var (
	cInterruptAlloc         = defineCode("E1080", "interrupt-handler-alloc", "export(interrupt) handler `{name}` may allocate or deallocate memory; interrupt handlers must never touch the allocator")
	cInterruptPanic         = defineCode("E1081", "interrupt-handler-panic", "export(interrupt) handler `{name}` may panic; interrupt handlers must never unwind")
	cInterruptUnsafeExtern  = defineCode("E1082", "interrupt-handler-unsafe-extern", "export(interrupt) handler `{name}` may block or reach unverified code; give the callee a body the compiler can check: `noalloc` alone is no interrupt contract, and an extern(C) declaration cannot carry one yet")
	cInterruptUnknown       = defineCode("E1083", "interrupt-handler-unknown", "export(interrupt) handler `{name}` reaches a call whose target is not known until run time; interrupt handlers may only call statically resolvable functions")
	cInterruptArgs          = defineCode("E1084", "interrupt-handler-args", "export(interrupt) handler `{name}` takes parameters; the generated wrapper cannot box them without allocating; poll `host.signals()` or a static flag from the handler instead")
	cInterruptReturn        = defineCode("E1085", "interrupt-handler-return", "export(interrupt) handler `{name}` must return `Unit`; the generated wrapper cannot box a return value without allocating")
	cInterruptExtern        = defineCode("E1086", "interrupt-extern", "`interrupt` belongs to `export` blocks; extern declarations have no body to check")
	cInterruptUnpermitted   = defineCode("E1088", "interrupt-handler-unpermitted", "export(interrupt) handler `{name}` reaches `{op}`, which its interrupt configuration does not permit; name it in the handler's `Interrupt` record in mod.kg, within the root record's limit")
	cInterruptAsm           = defineCode("E1089", "interrupt-handler-asm", "export(interrupt) handler `{name}` reaches assembly the compiler cannot verify; a reviewed trusted summary is required and none can be declared yet, so keep to register arithmetic, balanced push/pop and `call`/`jmp` to a checked `sym` operand")
	cInterruptLowering      = defineCode("E1090", "interrupt-handler-lowering", "export(interrupt) handler `{name}` lowers to code that {what}; the checker accepted the source, but the backend cannot run it without that operation, so keep the handler's values unboxed and its calls direct")
	cInterruptConfig        = defineCode("E1091", "interrupt-config-permission", "export(interrupt) handler `{name}`: the {origin} interrupt configuration names `{op}`, which {why}; a handler record only narrows the root `Interrupt` record and names operations exactly as their reviewed contract does")
	cInterruptConfigHandler = defineCode("E1092", "interrupt-config-handler", "the interrupt configuration names handler `{key}`, which is not an export(interrupt) function of this build; key a handler record as `pkg/path.name`")
	cInterruptDrop          = defineCode("E1087", "interrupt-handler-drop", "export(interrupt) handler `{name}` drops a value of type `{type}` in the handler; keep heap-owning values out of interrupt handlers")
)
