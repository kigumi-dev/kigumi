package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

// simdCallOps are the std/simd functions lowered without a runtime entry:
// their vector operands and result are direct-carried values, never boxes.
var simdCallOps = map[string]bool{
	"simd.Vec.splat":     true,
	"simd.Vec.fromArray": true,
	"simd.Vec.toArray":   true,
	"simd.Vec.add":       true,
	"simd.Vec.sub":       true,
	"simd.Vec.mul":       true,
}

// SimdCall reports whether in is one of those calls.
func (p *Program) SimdCall(in midir.Inst) bool {
	return in.Op == midir.OpCall && simdCallOps[opdef.StdKey(p.R, in.Ent, true)]
}
