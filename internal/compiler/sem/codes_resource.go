package sem

// Diagnostic catalog, E141x: the strict stack-profile gate (looked up by
// name from internal/compiler/mir) and the stable-address call-scoped lease.
var (
	cStackChainUnbounded = defineCode("E1410", "stack-chain-unbounded", "`{name}` is a strict-profile stack root whose call chain is unbounded: {detail}; bound the recursion, or build this entry under the hosted-growable default instead")
	cStackChainUnknown   = defineCode("E1411", "stack-chain-unknown", "`{name}` is a strict-profile stack root whose call chain cannot be proven bounded: {detail}; resolve the dynamic call, or give the callee a body this build can measure")
	cStackBudgetExceeded = defineCode("E1412", "stack-budget-exceeded", "`{name}`'s call chain needs at least {need} bytes, over the {budget}-byte strict stack budget")
	cCarrierLeaseEscape  = defineCode("E1413", "carrier-lease-escape", "`{fn}`'s callback returns `{ret}`, which can carry the leased pointer out past the call; keep the pointer inside the callback, or use a retained carrier (`ffi.Pin`) for data that must outlive this call")
	cAddressLeaseType    = defineCode("E1414", "address-lease-type", "`{type}` cannot be leased an address by `{fn}`: use a fixed-width number, `Bool`, `Char`, or a pointer; a record needs `CValue`'s own marshaled copy instead, since a record's boxed fields are not laid out as a C struct")
)
