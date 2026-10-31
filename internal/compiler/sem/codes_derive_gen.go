package sem

// Schema IR derive generator (internal/toolchain/contract/schema) failures, E1430 and E1432.
var (
	cDeriveGeneratorFailed      = defineCode("E1430", "derive-generator-failed", "`{name}`'s `{iface}` derive generator failed: {detail}; fix the declaration's shape so the generator can produce valid source")
	cDeriveGeneratorUnsupported = defineCode("E1432", "derive-generator-unsupported", "no derive generator is registered for `{name}`'s `{iface}`")
)
