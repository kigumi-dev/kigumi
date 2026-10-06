package sem

// Diagnostic catalog, E152x: a value formed over an abstract requirement
// with no single witness to run.
var (
	cRequirementMethodValue = defineCode("E1520", "requirement-method-value", "`{name}` is an interface requirement with no single implementation; call it directly instead of taking a method value")
)
