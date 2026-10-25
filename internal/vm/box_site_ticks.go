package vm

import (
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// A local is computed when native keeps it in a register: every definition is a computation, a register read, or a copy of one.
func (m *Machine) computedLocals(f *mir.Func, fixed, param, boxed []bool) []bool {
	defs := make([][]*mir.Inst, len(f.Locals))
	for _, blk := range f.Blocks {
		for i := range blk.Insts {
			if in := &blk.Insts[i]; in.Op.Yields() {
				defs[in.Dst] = append(defs[in.Dst], in)
			}
		}
	}
	out := make([]bool, len(f.Locals))
	register := func(l mir.LocalID) bool { return fixed[l] || out[l] || param[l] && !boxed[l] }
	for grew := true; grew; {
		grew = false
		for l, ds := range defs {
			if out[l] || len(ds) == 0 || f.Locals[l].Cell {
				continue
			}
			ok, computes := true, false
			for _, in := range ds {
				switch {
				case m.computes(f, in):
					computes = true
				case in.Op == mir.OpConst:
				case (in.Op == mir.OpAlias || in.Op == mir.OpShare || in.Op == mir.OpMove || in.Op == mir.OpCopy) && register(in.Args[0]):
					computes = computes || !fixed[in.Args[0]]
				default:
					ok = false
				}
			}
			if ok && computes {
				out[l], grew = true, true
			}
		}
	}
	return out
}

func (m *Machine) computes(f *mir.Func, in *mir.Inst) bool {
	switch in.Op {
	case mir.OpBinary, mir.OpUnary:
		return true
	case mir.OpField:
		return f.IsDirect(in.Args[0])
	case mir.OpBuiltin:
		t := f.Locals[in.Dst].Type
		return in.Str == "iter.at" && !f.IsDirect(in.Args[0]) && m.r.Types.IsInteger(t) && t != sem.TyI128 && t != sem.TyU128
	case mir.OpCall:
		g := m.p.ByEnt[in.Ent]
		return in.DPrim != mir.DPNone || m.p.FloatConversion(f, *in) || g != nil && len(g.Blocks) > 0 && !g.Async
	}
	return false
}

// Native boxes a computed scalar at the call when the callee keeps that parameter boxed.
func (m *Machine) callTicks(f *mir.Func, in *mir.Inst, loc []*obj) {
	g := m.p.ByEnt[in.Ent]
	if m.opts.FaultInjectAt <= 0 || in.Op != mir.OpCall || in.DPrim != mir.DPNone || g == nil || f.Wraps != nil {
		return
	}
	lf := m.factsOf(f)
	boxed := m.boxParams.Of(g)
	for i, a := range in.Args {
		if i >= len(boxed) || !boxed[i] || !lf.computed[a] && !(lf.param[a] && !lf.boxed[a]) || !scalarBoxes(loc[a]) {
			continue
		}
		m.tick(1)
	}
}

// Native allocates a header and a slot array, plus a box per computed scalar leaf.
func (m *Machine) aggTicks(f *mir.Func, in *mir.Inst, args []*obj, n int) {
	if m.opts.FaultInjectAt <= 0 || f.IsDirect(in.Dst) {
		return
	}
	lf := m.factsOf(f)
	if lf.fixed[in.Dst] {
		return
	}
	m.tick(n)
	for i, a := range in.Args {
		if lf.computed[a] {
			m.scalarTick(args[i])
		}
	}
}

// bindAllocs is the closure object, its environment array and the array's slots that native builds for a bind with witnesses.
const bindAllocs = 3

func (m *Machine) tick(n int) {
	for range n {
		if m.faultTick() {
			m.abort("out of memory")
		}
	}
}
