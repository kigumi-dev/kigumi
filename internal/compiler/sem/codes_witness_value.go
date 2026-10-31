package sem

// Diagnostic catalog, E152x: a value formed over an abstract requirement
// with no single witness to run.
var (
	cRequirementMethodValue = defineCode("E1520", "requirement-method-value", "`{name}` is an interface requirement with no single implementation; call it directly instead of taking a method value")
)

// Diagnostic catalog, E1800: a requirement method value that would consume
// a receiver the value does not own.
var (
	cRequirementMoveValue = defineCode("E1800", "requirement-move-method-value", "`{name}` consumes its receiver, so a method value on `{type}` could consume it twice; call it directly instead")
)
