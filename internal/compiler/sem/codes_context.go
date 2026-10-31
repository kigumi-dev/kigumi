package sem

// Diagnostic catalog, E121x: `context` provider keys.
var (
	cCtxWrongProviderType    = defineCode("E1210", "ctx-wrong-provider-type", "`{type}` does not declare any context keys; mark its type with `@provides(...)` or pass a value of a provider type")
	cCtxDuplicateBinding     = defineCode("E1211", "ctx-duplicate-binding", "context key `{key}` is already supplied by another provider in this `context (...)` list; remove one")
	cCtxUnboundKey           = defineCode("E1212", "ctx-unbound-key", "context key `{key}` is required here but nothing supplies it; wrap the call in `context (...) { }` supplying it")
	cCtxProvidesNotKey       = defineCode("E1213", "ctx-provides-not-key", "`{name}` in `@provides` is not a `context` key declaration")
	cCtxProvidesMissingField = defineCode("E1214", "ctx-provides-missing-field", "`{type}` has no field named `{key}` to supply that context key; add a field with that name")
	cCtxProvidesFieldType    = defineCode("E1215", "ctx-provides-field-type", "field `{key}` of `{type}` has type `{have}`, but the key is declared `{want}`")
	cCtxProvidesDuplicate    = defineCode("E1216", "ctx-provides-duplicate", "context key `{key}` is listed twice in `@provides`")
	cCtxValueReal            = defineCode("E1217", "ctx-value-real", "`{name}` requires context key `{key}` and can't be used as a plain function value; call it directly, or wrap the use in a closure inside a bound `context (...) { }`")
	cCtxVisibilityLeak       = defineCode("E1218", "ctx-visibility-leak", "public function `{name}` requires {vis} context key `{key}`, which callers outside its visibility cannot supply")
)
