package sem

// Metadata schema nominal declaration (`@schema`), E1350-E1369.
var (
	cSchemaWrongTarget  = defineCode("E1350", "schema-wrong-target", "`@schema` applies to type declarations only")
	cSchemaDuplicate    = defineCode("E1351", "schema-duplicate", "duplicate `@schema` attribute")
	cSchemaBadArg       = defineCode("E1352", "schema-bad-arg", "`@schema` takes up to three literal arguments (repeatable: Bool, ordered: Bool, target: String)")
	cDerivableDuplicate = defineCode("E1353", "derivable-duplicate", "duplicate `@derivable` attribute")
)
