package sem

// Diagnostic catalog, E2187..E2190: default arguments and pipeline placeholders.
var (
	cDefaultOrder      = defineCode("E2187", "default-param-order", "parameter `{name}` has no default but follows a parameter that has one; put every parameter with a default after the required ones")
	cDefaultNotAllowed = defineCode("E2188", "default-not-allowed", "default arguments are not allowed on {what}; write a wrapper function that supplies the value")
	cPipePlaceholder   = defineCode("E2189", "pipe-placeholder", "{what}; a `_` placeholder is one whole direct argument of the parenthesized call on the right of `|>`, so write `value |> (v) => f(g(v), v)` for anything else")
	cArgMissing        = defineCode("E2190", "call-missing-argument", "`{fn}` is missing the argument `{name}`, which has no default")
)
