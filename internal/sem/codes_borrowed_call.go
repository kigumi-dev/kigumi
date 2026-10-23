package sem

var cBorrowedCallKind = defineCode("E2233", "borrowed-call-kind", "`{type}` cannot be called through this borrow: {why}; call an `Fn` through `&`, an `FnMut` through `&mut`, and an `FnOnce` by value")

var cTempBorrowLet = defineCode("E2234", "temporary-borrow-let", "`let` cannot keep a borrow of {what}, which is dropped at the end of this statement; bind the value to a local first and borrow the local")

var cTempBorrowKept = defineCode("E2235", "temporary-borrow-kept", "`{name}` keeps a borrow of a temporary {where}; bind the temporary to a local first and borrow the local")
