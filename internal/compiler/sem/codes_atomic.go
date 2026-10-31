package sem

// Diagnostic catalog, E2248-E2250: atomic and volatile operations of std/mem.
var (
	cAtomicOrder = defineCode("E2248", "atomic-order", "{what}; write the order as a `mem.AtomicOrder` variant; a load takes Relaxed, Acquire or SeqCst, a store Relaxed, Release or SeqCst")
	cAtomicType  = defineCode("E2249", "atomic-type", "`{type}` cannot be the type of `{name}`; {allowed}")
	cAtomicPlace = defineCode("E2250", "atomic-place", "{what}; an atomic operation names a Kigumi-owned `static mut` of an integer type up to 64 bits or Bool as `&NAME` or `&mut NAME` (or is a method of an UnsafeCell), and is called directly")
)
