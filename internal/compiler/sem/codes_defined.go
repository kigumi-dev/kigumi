package sem

// Nominal defined types: `type A = T` identity, and the package-private
// `A(value)` wrap / `a.value` unwrap representation operations.
var (
	cDefinedCycle          = defineCode("E1170", "defined-type-cycle", "defined type `{name}`'s underlying type never bottoms out at a concrete representation")
	cDefinedWrapOutside    = defineCode("E1171", "defined-wrap-outside-package", "`{name}(...)` is a representation operation usable only inside the package that declares `{name}`; use the functions that package exports for `{name}` instead")
	cDefinedValueOutside   = defineCode("E1172", "defined-value-outside-package", "`.value` is `{name}`'s private representation, usable only inside the package that declares `{name}`; use the functions that package exports for `{name}` instead")
	cDefinedWrapArity      = defineCode("E1173", "defined-wrap-arity", "`{name}(...)` takes exactly one argument, {n} given")
	cDefinedWrapNamedArg   = defineCode("E1174", "defined-wrap-named-arg", "named, default and spread arguments are not allowed when wrapping `{name}`")
	cDefinedValueClash     = defineCode("E1175", "defined-value-clash", "`value` is `{name}`'s representation projection and cannot also be a method name")
	cAliasNotConstructible = defineCode("E1176", "alias-not-constructible", "`{name}` is a transparent alias with no representation constructor")
)
