package llgen

import (
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) isDirect(l mir.LocalID) bool {
	_, ok := e.direct[l]
	return ok
}

// In the boxed model, a write through one of these reaches the owner's
// storage, not a copy.
func readsForeign(op mir.Opcode) bool {
	switch op {
	case mir.OpCellGet, mir.OpIndex, mir.OpPayload, mir.OpUnbox, mir.OpGlobalGet,
		mir.OpField, mir.OpFieldMove:
		return true
	}
	return false
}

// A local Borrowed by one of these can still be native: copying it owns no
// reference to protect.
func copyDef(op mir.Opcode) bool {
	switch op {
	case mir.OpField, mir.OpFieldMove, mir.OpCopy, mir.OpAlias, mir.OpMove, mir.OpShare, mir.OpRecord:
		return true
	}
	return false
}

func copyDefOnly(f *mir.Func) []bool {
	out := make([]bool, len(f.Locals))
	seen := make([]bool, len(f.Locals))
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() {
				continue
			}
			out[in.Dst] = copyDef(in.Op) && (!seen[in.Dst] || out[in.Dst])
			seen[in.Dst] = true
		}
	}
	return out
}

func foreignLocals(f *mir.Func) []bool {
	out := make([]bool, len(f.Locals))
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || out[in.Dst] {
					continue
				}
				same := in.Op == mir.OpMove || in.Op == mir.OpAlias || (in.Op == mir.OpShare && !in.Copy)
				if readsForeign(in.Op) || (same && out[in.Args[0]]) {
					out[in.Dst], changed = true, true
				}
			}
		}
	}
	return out
}

// An async frame slot holds one pointer, so async bodies are excluded.
func (e *emitter) classifyDirect(f *mir.Func) map[mir.LocalID]*aggInfo {
	out := map[mir.LocalID]*aggInfo{}
	if f.Async {
		return out
	}
	isParam := make([]bool, len(f.Locals))
	for _, p := range f.Params {
		isParam[p] = true
	}
	copyDefined := copyDefOnly(f)
	for i, l := range f.Locals {
		if isParam[i] || l.Cell || framed(f, mir.LocalID(i)) {
			continue
		}
		if l.Borrowed && !copyDefined[i] {
			continue
		}
		if a, ok := e.agg(l.Type); ok {
			out[mir.LocalID(i)] = a
		}
	}
	for i, p := range f.Params {
		if a := e.sig.param(i); a != nil {
			out[p] = a
		}
	}
	cand := make([]bool, len(f.Locals))
	for i := range f.Locals {
		if _, ok := e.agg(f.Locals[i].Type); ok {
			cand[i] = true
		}
	}
	foreign := foreignLocals(f)
	for changed := true; changed; {
		changed = false
		drop := func(l mir.LocalID) {
			if _, ok := out[l]; ok {
				delete(out, l)
				changed = true
			}
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				classifyDirectUses(in, out, cand, foreign, drop)
			}
		}
	}
	return out
}

// A borrow needs an address and a write through an alias must land, so
// neither keeps its local native.
func classifyDirectUses(in mir.Inst, out map[mir.LocalID]*aggInfo, cand, foreign []bool, drop func(mir.LocalID)) {
	switch in.Op {
	case mir.OpAsm, mir.OpCallC, mir.OpShell:
		for _, a := range in.Args {
			drop(a)
		}
	case mir.OpBorrow:
		drop(in.Args[0])
		drop(in.Dst)
	case mir.OpNewCell:
		// The result's type is the content's, but it holds a cell object.
		drop(in.Args[0])
		drop(in.Dst)
	case mir.OpCellGet, mir.OpIndex, mir.OpPayload, mir.OpUnbox, mir.OpGlobalGet:
		drop(in.Dst)
	case mir.OpSetField:
		if foreign[in.Args[0]] {
			drop(in.Args[0])
		}
	case mir.OpField, mir.OpFieldMove:
		if cand[in.Dst] {
			agree(in, out, drop)
		}
	case mir.OpMove, mir.OpCopy, mir.OpShare, mir.OpAlias:
		if cand[in.Dst] || cand[in.Args[0]] {
			agree(in, out, drop)
		}
	}
}

// The boxed model hands back the same object; a native aggregate, a copy.
func agree(in mir.Inst, out map[mir.LocalID]*aggInfo, drop func(mir.LocalID)) {
	_, src := out[in.Args[0]]
	_, dst := out[in.Dst]
	if src != dst {
		drop(in.Args[0])
		drop(in.Dst)
	}
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
