package mir

import "kigumi/internal/sem"

// isrCheckedOps are the integer operators whose overflow or zero divisor traps.
var isrCheckedOps = map[string]bool{"+": true, "-": true, "*": true, "/": true, "%": true, "<<": true, ">>": true, "neg": true}

// isrForbidden names what in does that no interrupt handler may, "" when it is fine; releases selects the release pass.
func (p *Program) isrForbidden(f *Func, in Inst, inert []bool, releases bool) string {
	if in.Op == OpDrop || in.Op == OpRelease {
		if l := in.Args[0]; releases && !p.isrInertDrop(f, l, inert) {
			return "releases a `" + p.R.TypeString(f.Locals[l].Type) + "` that may free memory"
		}
		return ""
	}
	if releases {
		return ""
	}
	if t := p.boxedType(f, in); t != 0 {
		return "boxes a `" + p.R.TypeString(t) + "` value"
	}
	if t := p.boxedResult(f, in); t != 0 {
		return "receives a `" + p.R.TypeString(t) + "` result in a box allocated for it"
	}
	if p.allocCopy(f, in) {
		return "copies a value into a fresh allocation"
	}
	if in.NoTrap && (in.Op == OpPanic || in.Op == OpBinary) {
		return ""
	}
	switch in.Op {
	case OpPanic:
		return "may panic"
	case OpIndex, OpSetIndex:
		return "may panic on an out-of-bounds index"
	case OpBinary, OpUnary:
		if isrCheckedOps[in.Str] && in.Dst != 0 && p.R.Types.IsInteger(f.Locals[in.Dst].Type) {
			return "may panic on integer overflow or a zero divisor"
		}
	case OpBuiltin:
		if in.Str != "direct.eq" && in.Str != "direct.hash" {
			return "calls the runtime helper `" + in.Str + "`"
		}
	case OpShell, OpInterp:
		return "allocates a new value"
	case OpAwait, OpFutureNew, OpFutureDone, OpCancelled, OpCancelDone:
		return "suspends or builds a future"
	case OpCallC:
		return "calls through a C function pointer no contract covers"
	case OpRecord, OpArray, OpBox:
		if !f.IsDirect(in.Dst) {
			return "allocates a new `" + p.R.TypeString(f.Locals[in.Dst].Type) + "`"
		}
	case OpVariant, OpClosure:
		if len(in.Args) > 0 && !f.IsDirect(in.Dst) {
			return "allocates a new `" + p.R.TypeString(f.Locals[in.Dst].Type) + "`"
		}
	case OpNewCell:
		if !in.Scoped && !in.Lent {
			return "allocates a cell"
		}
	case OpCall:
		return p.isrCallForbidden(f, in)
	case OpCallValue:
		if p.isrValueTarget(f, in.Args[0]) == nil {
			return "calls a function value whose target the lowering cannot see"
		}
	}
	return ""
}

func (p *Program) isrInertDrop(f *Func, l LocalID, inert []bool) bool {
	t := f.Locals[l].Type
	return t == sem.TyUnit || p.R.RegScalar(t) || f.IsDirect(l) || p.isrImmortal(f, l, inert)
}

// isrInertArgs marks the parameters of g that the call in passes an immortal value.
func (p *Program) isrInertArgs(f *Func, in Inst, g *Func, inert []bool) []bool {
	if in.Op != OpCall || len(in.Args) != len(g.Params) {
		return nil
	}
	var out []bool
	for i, a := range in.Args {
		if p.isrImmortal(f, a, inert) {
			if out == nil {
				out = make([]bool, len(g.Params))
			}
			out[i] = true
		}
	}
	return out
}

func isrInertParam(f *Func, l LocalID, inert []bool) bool {
	i := isrParamIndex(f, l)
	return i >= 0 && i < len(inert) && inert[i]
}

func isrParamIndex(f *Func, l LocalID) int {
	for i, q := range f.Params {
		if q == l {
			return i
		}
	}
	return -1
}

type isrVisit struct {
	f     *Func
	inert string
}

func isrVisitKey(f *Func, inert []bool) isrVisit {
	b := make([]byte, len(inert))
	for i, v := range inert {
		b[i] = '0'
		if v {
			b[i] = '1'
		}
	}
	return isrVisit{f, string(b)}
}

// isrCallForbidden rejects a call the backend runs through a boxing runtime path or a foreign symbol.
func (p *Program) isrCallForbidden(f *Func, in Inst) string {
	name := p.R.Entity(in.Ent).Name
	if p.R.Entity(in.Ent).Kind == sem.EntFn && p.R.Fn(in.Ent).Abi != "" {
		return "calls the foreign function `" + name + "`"
	}
	if !p.IsStdPrimitive(in.Ent) {
		if p.ByEnt[in.Ent] == nil {
			return "calls `" + name + "`, whose lowered body is not available to verify"
		}
		return ""
	}
	if in.DPrim != DPNone {
		return ""
	}
	if t := f.Locals[in.Dst].Type; t != sem.TyUnit {
		return "calls `" + name + "` through the runtime, which returns its `" + p.R.TypeString(t) + "` result in a fresh box"
	}
	return ""
}
