package sem

// A generic fn item used where a borrowing callable type is expected.
var cGenericCallableBorrow = defineCode("E1130", "generic-callable-borrow", "`{name}`'s type parameter would be inferred as a reference here; declare the parameter `&T` or `&mut T` instead of relying on inference to borrow it")
