package sem

// Diagnostic catalog, E133x: the `mut self` receiver.
var (
	cAsyncSelfAssign     = defineCode("E1330", "async-self-assign", "`{name}` assigns `self` as a whole in an `async` body, where the caller's storage is out of reach by the time it runs; assign self's fields instead, or make the method synchronous")
	cBorrowedRecvReplace = defineCode("E1331", "borrowed-recv-replace", "`{name}` replaces its receiver as a whole, which a borrowed receiver cannot carry back to its owner; call it on the value itself, or on a field or element of one")
	cWitnessRecvReplace  = defineCode("E1332", "witness-recv-replace", "`{name}` on a borrowed type parameter can be witnessed by `{m}`, which replaces its receiver as a whole; take the receiver by value (`mut g: G`) rather than `&mut G`")
)
