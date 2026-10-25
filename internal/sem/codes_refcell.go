package sem

// Diagnostic catalog, E150x: a `&mut` borrow with no owner for a value it must carry back.
var (
	cRefCellNoOwner   = defineCode("E1500", "ref-cell-no-owner", "`{name}` is a borrow bound to a name, with no owner a whole new value could go back to; write through the place itself, or pass `&mut` straight into the call")
	cRefCellAsyncArg  = defineCode("E1501", "ref-cell-async-arg", "a `&mut` the callee may replace as a whole cannot be lent to an `async` call, which outlives the storage the new value would go back to; pass the value itself, or make the call synchronous")
	cRefCellLentRecv  = defineCode("E1502", "ref-cell-lent-recv", "`{name}` replaces its receiver as a whole, and this receiver is lent with no owner to return it to; call it on the value itself, or on a field or element of one")
	cRefCellLentValue = defineCode("E1503", "ref-cell-lent-value", "{what} is lent here, and the callee may replace its referent as a whole with no owner to return the new value to; borrow the place at the call with `&mut`, or pass the value itself")
	cAsyncValueArity  = defineCode("E1504", "async-value-arity", "`{name}` takes {count} arguments, more than the 31 a call through an `async` fn value can lend; call it directly, or pass some of the arguments in a record")
)
