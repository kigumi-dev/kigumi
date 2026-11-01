package sem

// E3xx: generics and conformance, then the E10xx const generic codes.
var (
	cConstraintUnsatisfied = defineCode("E300", "constraint-unsatisfied", "`{type}` does not satisfy `{constraint}`: {missing}")
	cInferUnresolved       = defineCode("E301", "infer-unresolved", "cannot infer type parameter `{name}`; write {hint}")
	cNomonoRecursion       = defineCode("E302", "nomono-recursion", "instantiation of `{f}` requires `{g}`: polymorphic recursion does not terminate")
	cParamNoMember         = defineCode("E303", "param-no-member", "`{param}` has no method `{name}`; add a constraint")
	cImplUnsatisfied       = defineCode("E304", "impl-unsatisfied", "`{type}` does not implement `{iface}`: {missing}")
	cImplGenericUnsat      = defineCode("E305", "impl-generic-unsatisfied", "`{type}` does not implement `{iface}` for every `{param}`: {missing}")
	cTypeOccurs            = defineCode("E306", "type-occurs", "infinite type: `{a}` occurs in `{b}`")
	cIfaceNotObjectSafe    = defineCode("E307", "iface-not-object-safe", "`{iface}` is not object-safe ({why}); use it as a constraint")
	cIfaceCoerceUnsat      = defineCode("E308", "iface-coerce-unsatisfied", "`{type}` does not implement `{iface}`: {missing}")

	cConstParamType       = defineCode("E1004", "const-param-type", "`{type}` cannot be a const parameter's type; use usize, a fixed-width integer, Int or Bool")
	cConstExprUnsupported = defineCode("E1005", "const-expr-unsupported", "const generic arguments must be a literal, a const declaration or another const parameter; expressions like `N + 1` are not supported yet")
	cConstArgKind         = defineCode("E1006", "const-arg-kind", "`{name}` expects {want} argument here, not {got}")
	cExportConstParam     = defineCode("E1007", "export-const-param", "`{name}` is exported to C and cannot have a const parameter")
)
