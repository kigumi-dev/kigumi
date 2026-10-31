package vm

import (
	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

// closurePlan is what a closure-building instruction decides the same way every time.
type closurePlan struct {
	req        *reqCall
	deny       string
	useLowered bool
	f          *mir.Func
	key        string
	lent       int
	dyn        sem.TypeID
}

func (m *Machine) closurePlanFor(in *mir.Inst, ic *instCode, ncaps int) *closurePlan {
	if p := ic.clo.Load(); p != nil {
		return p
	}
	p := m.planClosure(in, ncaps)
	ic.clo.Store(p)
	return p
}

func (m *Machine) planClosure(in *mir.Inst, ncaps int) *closurePlan {
	e := m.r.Entity(in.Ent)
	if in.Op == mir.OpClosure && e.Kind == sem.EntFn && (m.r.Entity(e.Parent).Kind == sem.EntInterface || in.Str == "applied") {
		return &closurePlan{req: &reqCall{name: e.Name, witness: in.Str == "witness" || in.Str == "applied", move: m.r.Fn(in.Ent).Recv == sem.RecvMove}}
	}
	f, lowered := m.lowered(in.Ent)
	p := &closurePlan{useLowered: lowered}
	if e.Kind == sem.EntFn && e.Flags&sem.EfStd != 0 {
		key := m.stdKey(in.Ent)
		if m.opts.Sandbox && hostEffect(key) {
			p.deny = "`" + key + "` is not available at compile time"
		}
		if _, ok := m.hostFor(in.Prov, key); ok && m.hostWins(lowered) {
			p.useLowered = false
		}
	}
	if p.useLowered {
		p.f = f
		return p
	}
	p.key = m.stdKey(in.Ent)
	p.lent = mir.LentMask(m.r, in.Ent)
	if in.Op == mir.OpClosure {
		p.lent >>= ncaps
	}
	p.dyn = m.curFn.Locals[in.Dst].Type
	return p
}
