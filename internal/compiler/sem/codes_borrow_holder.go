package sem

// E158x: a borrow in a variant payload or array element is released with
// its holder (like borrow-owned-field) and has no count to give back.
var (
	cBorrowInPayload    = defineCode("E1580", "borrow-in-payload", "a borrow cannot go in {what}, a payload of `{type}`: a payload is released with the variant and this borrow has no count to give; nest it in a type that takes a `['a]` and routes the borrow through its own field, or store a value it can own")
	cBorrowInElement    = defineCode("E1581", "borrow-in-element", "a borrow cannot go in {what}, an element of `{type}`: an element is released with the array and this borrow has no count to give; an array tracks a borrow only inside a value whose type takes a `['a]`, so wrap it in such a type, or store a value it can own")
	cBorrowOwnedPayload = defineCode("E1582", "borrow-owned-payload", "a borrow cannot go in {what}, a payload of `{type}`: a payload declared that way is released with the variant, and this borrow has no count to give; nest it in a type that takes a `['a]` and routes the borrow through its own field, or store a value it can own")
)

var (
	cBorrowStoreShared    = defineCode("E2158", "borrow-store-shared", "a value borrowing {name} cannot go into shared storage: another handle to that storage can keep it after {name} is gone; finish with the value first (await a Future before storing what it returns), or store a value that borrows nothing")
	cBorrowStoreOutlives  = defineCode("E2159", "borrow-store-outlives", "a borrow cannot be stored into `{name}` here: it belongs to the caller and outlives this call; only a value carrying this function's own lifetime, the one that place's type names, may go in it, so build the container here and return it instead")
	cBorrowStoreUntracked = defineCode("E2160", "borrow-store-untracked", "a borrow cannot be stored into {type}: the checker does not track where its contents borrow from; it follows writes only into an array, fixed array, map, record or variant held in a local, a `&mut` parameter, or a reference bound once to one of those, so write through that local or store a value it can own")
	cBorrowStoreOpaque    = defineCode("E2161", "borrow-store-opaque-call", "a call through a function value may store a borrow into `{name}`, and that write cannot be tracked; call the function by name, or pass a value it can own")
	cBorrowMultiLifetime  = defineCode("E2162", "borrow-multi-lifetime", "a borrow inside `{type}` cannot be traced here: that type names more than one lifetime, and a borrow is traced across calls under a single one; give the element type a single `'a` of its own, or keep each lifetime in a separate value")
)

var cMutFieldLentTwice = defineCode("E2293", "mut-field-lent-twice", "`{place}` is lent to this call as `&mut`, and another argument or the receiver of the same call reaches that value too; the callee would hold two paths to it; pass it once, or read the other value before the call")

var cSharedMutRef = defineCode("E2292", "shared-mut-ref", "`{place}` is a `&mut` reached through a shared reference, so it cannot be copied out or written through: another path to the same value can still use it; read through it, or take the holder with `&mut` or `mut self`")

var cLifetimeMixedCall = defineCode("E2315", "lifetime-mixed-call", "a borrow returned through this call may come from `{name}`, a local, or from a borrow a parameter carries in, and the checker cannot trace the two together; pass the call only one kind, for instance by binding a reference read from a parameter to a local first")

var cBorrowCallerLifetime = defineCode("E2320", "borrow-caller-lifetime", "a value borrowing {name} cannot be passed as `{type}`: `{lifetime}` belongs to the caller and outlives this call; pass a borrow the caller lent, or give the function value a plain `&` parameter, which cannot keep the borrow")
