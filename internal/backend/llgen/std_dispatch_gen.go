package llgen

import (
	"kigumi/internal/backend/llgen/runtimegen"
	opdef "kigumi/internal/compiler/mir/ops"
)

//go:generate env UPDATE=1 go test -run TestStdDispatchFresh .

// StdDispatch renders runtime/rt_std_dispatch.inc from the op registry and the
// C bodies in runtimegen/std_dispatch_bodies.txt.
func StdDispatch(reg *opdef.StdOpRegistry) (string, error) {
	return runtimegen.StdDispatch(runtimeOps(reg))
}
