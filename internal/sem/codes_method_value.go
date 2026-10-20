package sem

var (
	cMethodValueBorrowed = defineCode("E2163", "method-value-borrowed-receiver", "a method value on move-only `{type}` takes its receiver, but this receiver is borrowed here; call the method directly, or write a lambda")
	cMethodValuePlace    = defineCode("E2164", "method-value-place-receiver", "a method value on move-only `{type}` takes its receiver, which cannot be moved out of a field, element or static; bind the value to a local first, or call the method directly")
	cMethodValueAsync    = defineCode("E2165", "method-value-async-owned", "an `async` method value cannot take its move-only `{type}` receiver; call the method directly, or write a lambda")
)
