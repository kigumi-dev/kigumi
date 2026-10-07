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

// A box made for a direct local at a consuming position is not released again.
func (e *emitter) aggConsumes(in mir.Inst, i int) bool {
	switch in.Op {
	case mir.OpRecord, mir.OpVariant, mir.OpArray, mir.OpBox, mir.OpGlobalSet, mir.OpNewCell,
		mir.OpDrop, mir.OpRelease, mir.OpAwait:
		return true
	case mir.OpSetField, mir.OpCellSet:
		return i == 1
	case mir.OpSetIndex:
		return i == 2
	case mir.OpCallValue:
		return i > 0
	case mir.OpCall:
		return !e.p.ArgLent(in, i)
	}
	return false
}

func (e *emitter) releaseAggArgs(sb *strings.Builder, in mir.Inst, args []string, boxed []bool) {
	for i := range in.Args {
		if boxed[i] && !e.aggConsumes(in, i) {
			e.irb(sb).Call(rtRelease, vptr(args[i]))
		}
	}
}
