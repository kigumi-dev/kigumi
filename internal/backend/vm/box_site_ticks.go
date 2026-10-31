package vm

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// A local is computed when native keeps it in a register: every definition is a computation, a register read, or a copy of one.
func (m *Machine) computedLocals(f *midir.Func, fixed, param, boxed []bool) []bool {
	defs := make([][]*midir.Inst, len(f.Locals))
	for _, blk := range f.Blocks {
		for i := range blk.Insts {
			if in := &blk.Insts[i]; in.Op.Yields() {
				defs[in.Dst] = append(defs[in.Dst], in)
			}
		}
	}
	out := make([]bool, len(f.Locals))
	register := func(l midir.LocalID) bool { return fixed[l] || out[l] || param[l] && !boxed[l] }
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
				case in.Op == midir.OpConst:
				case (in.Op == midir.OpAlias || in.Op == midir.OpShare || in.Op == midir.OpMove || in.Op == midir.OpCopy) && register(in.Args[0]):
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

func (m *Machine) computes(f *midir.Func, in *midir.Inst) bool {
	switch in.Op {
	case midir.OpBinary, midir.OpUnary:
		return true
	case midir.OpField:
		return f.IsDirect(in.Args[0])
	case midir.OpBuiltin:
		t := f.Locals[in.Dst].Type
		return in.Str == "iter.at" && !f.IsDirect(in.Args[0]) && m.r.Types.IsInteger(t) && t != sem.TyI128 && t != sem.TyU128
	case midir.OpCall:
		g := m.p.ByEnt[in.Ent]
		return in.DPrim != opdef.DPNone || m.p.FloatConversion(f, *in) || g != nil && len(g.Blocks) > 0 && !g.Async
	}
	return false
}

// Native boxes a computed scalar at the call when the callee keeps that parameter boxed.
func (m *Machine) callTicks(f *midir.Func, in *midir.Inst, loc []*obj) {
	g := m.p.ByEnt[in.Ent]
	if m.opts.FaultInjectAt <= 0 || in.Op != midir.OpCall || in.DPrim != opdef.DPNone || g == nil || f.Wraps != nil {
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
func (m *Machine) aggTicks(f *midir.Func, in *midir.Inst, args []*obj, n int) {
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
