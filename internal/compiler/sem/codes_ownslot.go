package sem

// Diagnostic catalog, E153x: a borrow handed to a slot that owns what it holds.
var (
	cBorrowOwnedField = defineCode("E1531", "borrow-owned-field", "a borrow cannot go in `{name}`, a field of `{type}` declared `{type2}`: a field declared that way is released with the record, and this borrow has no count to give; declare the field `&'a T` and take a `['a]` on the record, or store a value the record can own")
	cBorrowOwnedParam = defineCode("E1532", "borrow-owned-param", "a borrow cannot go to `{name}`, a parameter of `{name2}` declared `{type}`: a parameter declared that way is dropped by the callee, and this borrow has no count to give; declare the parameter `&T`, or pass a value the callee can own")
)
