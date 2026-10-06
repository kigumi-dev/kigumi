package sem

// E158x: a borrow in a variant payload or array element is released with
// its holder (like borrow-owned-field) and has no count to give back.
var (
	cBorrowInPayload    = defineCode("E1580", "borrow-in-payload", "a borrow cannot go in {what}, a payload of `{type}`: a payload is released with the variant and this borrow has no count to give; nest it in a type that takes a `['a]` and routes the borrow through its own field, or store a value it can own")
	cBorrowInElement    = defineCode("E1581", "borrow-in-element", "a borrow cannot go in {what}, an element of `{type}`: an element is released with the array and this borrow has no count to give; an array cannot take a `['a]` of its own, so store a value it can own")
	cBorrowOwnedPayload = defineCode("E1582", "borrow-owned-payload", "a borrow cannot go in {what}, a payload of `{type}`: a payload declared that way is released with the variant, and this borrow has no count to give; nest it in a type that takes a `['a]` and routes the borrow through its own field, or store a value it can own")
)
