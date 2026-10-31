package vm

import (
	"sync/atomic"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

// funcCode holds per-instruction facts a run resolves once per function.
type funcCode struct {
	blocks [][]instCode
}

type instCode struct {
	bin        bop
	nk         int
	konst      *obj
	call       atomic.Pointer[callPlan]
	clo        atomic.Pointer[closurePlan]
	inlineCopy bool
}

type planKind uint8

const (
	planSlow planKind = iota
	planLowered
	planHost
	planStd
)

type callPlan struct {
	kind    planKind
	f       *mir.Func
	code    *funcCode
	host    hostFn
	ent     sem.EntityID
	key     string
	lowered bool
}

var slowPlan = &callPlan{kind: planSlow}

func (m *Machine) codeFor(f *mir.Func) *funcCode {
	if c, ok := m.sh.codes.Load(f); ok {
		return c.(*funcCode)
	}
	c, _ := m.sh.codes.LoadOrStore(f, m.decode(f))
	return c.(*funcCode)
}

func (m *Machine) decode(f *mir.Func) *funcCode {
	c := &funcCode{blocks: make([][]instCode, len(f.Blocks))}
	for b := range f.Blocks {
		insts := f.Blocks[b].Insts
		ics := make([]instCode, len(insts))
		for k := range insts {
			in, ic := &insts[k], &ics[k]
			switch in.Op {
			case mir.OpBinary:
				ic.bin = bopOf(in.Str)
			case mir.OpUnary:
				if in.Str == "cast" {
					ic.nk = m.numKind(in.Type)
				}
			case mir.OpConst:
				ic.konst = m.scalarConst(in)
			}
		}
		c.blocks[b] = ics
	}
	for _, store := range m.p.CopyStores(f) {
		c.blocks[store.Block][store.Share].inlineCopy = true
	}
	return c
}

// scalarConst is the shared immortal a scalar literal evaluates to; strings and bytes stay fresh.
func (m *Machine) scalarConst(in *mir.Inst) *obj {
	v := m.constant(in)
	switch v.k {
	case kInt, kFloat, kChar:
		m.immortalize(v)
		return v
	case kBool, kUnit:
		return v
	}
	m.release(v)
	return nil
}

func (m *Machine) planFor(in *mir.Inst, ic *instCode) *callPlan {
	if p := ic.call.Load(); p != nil {
		return p
	}
	p := m.planCall(in)
	if p == nil {
		p = slowPlan
	}
	ic.call.Store(p)
	return p
}

// planCall mirrors callEntity's choice; nil leaves the call to callEntity.
func (m *Machine) planCall(in *mir.Inst) *callPlan {
	ent := in.Ent
	e := m.r.Entity(ent)
	isFn := e.Kind == sem.EntFn
	if isFn {
		info := m.r.Fn(ent)
		if info.Abi != "" || info.Naked || m.r.Entity(e.Parent).Kind == sem.EntInterface {
			return nil
		}
	}
	f, lowered := m.lowered(ent)
	if isFn && e.Flags&sem.EfStd != 0 {
		key := m.stdKey(ent)
		if m.opts.Sandbox && hostEffect(key) {
			return nil
		}
		if h, ok := m.hostFor(in.Prov, key); ok && m.hostWins(lowered) {
			return &callPlan{kind: planHost, host: h, ent: ent, lowered: lowered}
		}
	}
	if lowered {
		return &callPlan{kind: planLowered, f: f, code: m.codeFor(f)}
	}
	return &callPlan{kind: planStd, key: m.stdKey(ent)}
}

func (m *Machine) gather(loc []*obj, ids []mir.LocalID) []*obj {
	out := m.argSlice(len(ids))
	for i, a := range ids {
		out[i] = loc[a]
	}
	return out
}

const argSlabSize = 64

// argSlice carves from a chunk and never reuses it, since a callee may keep its arguments.
func (m *Machine) argSlice(n int) []*obj {
	if n > argSlabSize/4 {
		return make([]*obj, n)
	}
	if len(m.slab) < n {
		m.slab = make([]*obj, argSlabSize)
	}
	s := m.slab[:n:n]
	m.slab = m.slab[n:]
	return s
}
