package sem

// E158x: a borrow in a variant payload or array element is released with
// its holder (like borrow-owned-field) and has no count to give back.
var (
	cBorrowInPayload    = defineCode("E1580", "borrow-in-payload", "a borrow cannot go in {what}, a payload of `{type}`: a payload is released with the variant and this borrow has no count to give; nest it in a type that takes a `['a]` and routes the borrow through its own field, or store a value it can own")
	cBorrowInElement    = defineCode("E1581", "borrow-in-element", "a borrow cannot go in {what}, an element of `{type}`: an element is released with the array and this borrow has no count to give; an array tracks a borrow only inside a value whose type takes a `['a]`, so wrap it in such a type, or store a value it can own")
	cBorrowOwnedPayload = defineCode("E1582", "borrow-owned-payload", "a borrow cannot go in {what}, a payload of `{type}`: a payload declared that way is released with the variant, and this borrow has no count to give; nest it in a type that takes a `['a]` and routes the borrow through its own field, or store a value it can own")
)

var (
	cBorrowStoreOutlives  = defineCode("E2159", "borrow-store-outlives", "a borrow cannot be stored into `{name}` here: it belongs to the caller and outlives this call; only a value carrying this function's own lifetime, the one that place's type names, may go in it, so build the container here and return it instead")
	cBorrowStoreUntracked = defineCode("E2160", "borrow-store-untracked", "a borrow cannot be stored into {type}: the checker does not track where its contents borrow from; it follows writes only into an array, fixed array, record or variant held in a local or a `&mut` parameter, not through a named reference or a pattern binding, so write through that local or store a value it can own")
	cBorrowStoreOpaque    = defineCode("E2161", "borrow-store-opaque-call", "a call through a function value may store a borrow into `{name}`, and that write cannot be tracked; call the function by name, or pass a value it can own")
)
