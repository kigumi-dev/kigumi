package llgen

import (
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) isDirect(l mir.LocalID) bool {
	_, ok := e.direct[l]
	return ok
}

// classifyDirect reads the plan MIR already fixed: which locals stay
// unboxed, and where a crossing into the boxed model is.
func (e *emitter) classifyDirect(f *mir.Func) map[mir.LocalID]*aggInfo {
	out := map[mir.LocalID]*aggInfo{}
	for i := range f.Locals {
		if !f.IsDirect(mir.LocalID(i)) {
			continue
		}
		if a, ok := e.agg(f.Locals[i].Type); ok {
			out[mir.LocalID(i)] = a
		}
	}
	return out
}

// Only a crossing's own local is given back here: every other box belongs
// to the ownership plan.
func boxTemps(f *mir.Func) map[mir.LocalID]bool {
	out := map[mir.LocalID]bool{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpBoxDirect && f.Locals[in.Dst].Cross {
				out[in.Dst] = true
			}
		}
	}
	return out
}

func (e *emitter) crossSources(f *mir.Func) map[mir.LocalID]mir.LocalID {
	out := map[mir.LocalID]mir.LocalID{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpUnboxDirect && f.Locals[in.Dst].Cross && e.p.CrossHandsOver(f, &in) && !f.Locals[in.Args[0]].Cross {
				out[in.Dst] = in.Args[0]
			}
		}
	}
	return out
}

func (e *emitter) inst(sb *strings.Builder, f *mir.Func, in mir.Inst) {
	e.instBody(sb, f, in)
	e.releaseCrossSources(sb, f, in)
}

func (e *emitter) releaseCrossSources(sb *strings.Builder, f *mir.Func, in mir.Inst) {
	if in.Op == mir.OpShare {
		return
	}
	for i, a := range in.Args {
		if src, ok := e.crossSrc[a]; ok && e.p.ArgConsumed(f, in, i) {
			e.irb(sb).Call(rtRelease, vptr(e.load(sb, src)))
		}
	}
}

func (e *emitter) releaseAggArgs(sb *strings.Builder, in mir.Inst, args []string, boxed []bool) {
	for i := range in.Args {
		if boxed[i] && !e.p.BoxConsumed(in, i) {
			e.unslot(sb, in.Args[i])
			e.irb(sb).Call(rtRelease, vptr(args[i]))
		}
	}
}
