package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

// simdCallOps are the std/simd functions the native backend lowers at their
// call site; their vector arguments and result stay in registers, never boxes.
var simdCallOps = map[string]bool{
	"simd.Vec.splat":     true,
	"simd.Vec.fromArray": true,
	"simd.Vec.toArray":   true,
	"simd.Vec.add":       true,
	"simd.Vec.sub":       true,
	"simd.Vec.mul":       true,
}

// isSimdCall reports whether in calls one of those lowered functions.
func (p *planner) isSimdCall(in midir.Inst) bool {
	return in.Op == midir.OpCall && simdCallOps[opdef.StdKey(p.R, in.Ent, true)]
}
