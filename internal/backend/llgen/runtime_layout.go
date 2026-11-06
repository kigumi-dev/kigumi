package llgen

import "kigumi/internal/backend/llgen/runtimegen"

// RuntimeLayout is the normalized text of the value layout the runtime and
// the emitted code share; header is the emitted module's own view of it.
func RuntimeLayout() (string, error) { return runtimegen.Layout(header) }
