package llgen

import "kigumi/internal/backend/llgen/runtimegen"

// FreestandingSymbols is the allow-list of symbols a `--sys none` archive
// may leave undefined (internal/toolchain/driver checks real archives against it).
func FreestandingSymbols() map[string]bool { return runtimegen.FreestandingSymbols() }
