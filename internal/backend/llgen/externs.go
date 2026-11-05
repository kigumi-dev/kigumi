package llgen

// externs is rendered from runtimeFns (runtime_decls.go) so the two never
// drift apart.
var externs = renderExterns()
