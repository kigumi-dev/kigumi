package llgen

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

func (e *emitter) isDirect(l midir.LocalID) bool {
	_, ok := e.direct[l]
	return ok
}

// classifyDirect reads the plan MIR already fixed: which locals stay
// unboxed, and where a crossing into the boxed model is.
func (e *emitter) classifyDirect(f *midir.Func) map[midir.LocalID]*aggInfo {
	out := map[midir.LocalID]*aggInfo{}
	for i := range f.Locals {
		if !f.IsDirect(midir.LocalID(i)) {
			continue
		}
		if a, ok := e.agg(f.Locals[i].Type); ok {
			out[midir.LocalID(i)] = a
		}
	}
	return out
}

// Only a crossing's own local is given back here: every other box belongs
// to the ownership plan.
func boxTemps(f *midir.Func) map[midir.LocalID]bool {
	out := map[midir.LocalID]bool{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpBoxDirect && f.Locals[in.Dst].Cross {
				out[in.Dst] = true
			}
		}
	}
	return out
}

func (e *emitter) crossSources(f *midir.Func) map[midir.LocalID]midir.LocalID {
	out := map[midir.LocalID]midir.LocalID{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpUnboxDirect && f.Locals[in.Dst].Cross && e.p.CrossHandsOver(f, &in) && !f.Locals[in.Args[0]].Cross {
				out[in.Dst] = in.Args[0]
			}
		}
	}
	return out
}

// A crossing holds its own counts, so its source is released before the reader runs and a panic there leaks nothing.
func (e *emitter) inst(sb *strings.Builder, f *midir.Func, in midir.Inst) {
	e.releaseCrossSources(sb, f, in)
	e.settleCountedSources(sb, f, in)
	e.instBody(sb, f, in)
	e.settleCountedCross(sb, f, in)
}

func (e *emitter) releaseCrossSources(sb *strings.Builder, f *midir.Func, in midir.Inst) {
	if in.Op == midir.OpShare {
		return
	}
	for i, a := range in.Args {
		if src, ok := e.crossSrc[a]; ok && e.p.ArgConsumed(f, in, i) {
			e.irb(sb).Call(rtRelease, vptr(e.load(sb, src)))
		}
	}
}

func (e *emitter) releaseAggArgs(sb *strings.Builder, in midir.Inst, args []string, boxed []bool) {
	for i := range in.Args {
		if boxed[i] && !e.p.BoxConsumed(in, i) {
			e.unslot(sb, in.Args[i])
			e.irb(sb).Call(rtRelease, vptr(args[i]))
		}
	}
}
