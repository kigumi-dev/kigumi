package sem

// Schema IR derive generator (internal/schema) failures, E1430-E1433.
var (
	cDeriveGeneratorFailed           = defineCode("E1430", "derive-generator-failed", "`{name}`'s `{iface}` derive generator failed: {detail}; fix the declaration's shape so the generator can produce valid source")
	cDeriveGeneratorNoSchema         = defineCode("E1431", "derive-generator-no-schema", "`{name}` has no Schema IR declaration to derive `{iface}` from: {detail}")
	cDeriveGeneratorUnsupported      = defineCode("E1432", "derive-generator-unsupported", "no derive generator is registered for `{name}`'s `{iface}`")
	cDeriveGeneratorNondeterministic = defineCode("E1433", "derive-generator-nondeterministic", "`{name}`'s `{iface}` derive generator produced different source on a second run: {detail}; a generator must be a pure function of the declaration")
)
