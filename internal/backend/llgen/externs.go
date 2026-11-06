package llgen

import "kigumi/internal/backend/llgen/ir"

// externs is rendered from runtimeFns (runtime_decls.go) so the two never
// drift apart.
var externs = ir.RenderExterns(runtimeFns)
