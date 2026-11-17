package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
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

// SimdOpen reports the result type of a SIMD call whose lane type or count is still a parameter, which only an
// instance of f can lower.
func (p *Program) SimdOpen(f *midir.Func, in midir.Inst) (sem.TypeID, bool) {
	if !p.SimdCall(in) {
		return 0, false
	}
	t := f.Locals[in.Dst].Type
	elem, _, ok := p.R.SimdVecShape(t)
	if !ok {
		elem, _, ok = p.R.FixedElem(t)
	}
	return t, !ok || !p.R.SimdElem(elem)
}
