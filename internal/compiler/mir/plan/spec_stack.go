package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Two leaves of at most 8 bytes always fit the native ABI's 16-byte by-value limit; past it a value is copied into the frame.
const regLeaves = 2

// maxLeafWalkDepth bounds the leaf count walk over nested aggregates.
const maxLeafWalkDepth = 8

// The shared body holds a wide value in one boxed word per level.
func (s *specializer) deepens(sp *spec) bool {
	return sp.fn.Proto == "" && sp.held == nil && s.holdsWide(sp) && s.recursive(sp)
}

func (s *specializer) holdsWide(sp *spec) bool {
	f, g := sp.fn, s.p.ByEnt[sp.gen]
	for l := range g.Locals {
		if g.Locals[l].Type != f.Locals[l].Type && f.IsDirect(midir.LocalID(l)) && s.wide(f.Locals[l].Type) {
			return true
		}
	}
	return false
}

func (s *specializer) wide(t sem.TypeID) bool {
	n := s.leaves(t, 0)
	return n < 0 || n > regLeaves
}

func (s *specializer) leaves(t sem.TypeID, depth int) int {
	r := s.p.R
	switch {
	case depth > maxLeafWalkDepth:
		return -1
	case t == sem.TyUnit:
		return 0
	case !r.DirectAgg(t):
		return 1
	case r.IsFixedArray(t):
		return -1
	case r.DirectAdt(t):
		most := 0
		for _, v := range r.AggVariants(t) {
			n := s.sumLeaves(r.AggPayload(t, v), depth+1)
			if n < 0 {
				return -1
			}
			most = max(most, n)
		}
		return most + 1
	}
	return s.sumLeaves(r.AggFields(t), depth+1)
}

func (s *specializer) sumLeaves(ts []sem.TypeID, depth int) int {
	n := 0
	for _, t := range ts {
		k := s.leaves(t, depth)
		if k < 0 {
			return -1
		}
		n += k
	}
	return n
}

func (s *specializer) recursive(sp *spec) bool {
	seen := map[*midir.Func]bool{}
	work := []*midir.Func{sp.fn}
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				g := s.p.ByEnt[in.Ent]
				if !namesBody(in.Op) || g == nil || seen[g] {
					continue
				}
				if g == sp.fn {
					return true
				}
				seen[g] = true
				work = append(work, g)
			}
		}
	}
	return false
}

// A body is reached by a call, and by the function value, closure or method value that names it.
func namesBody(op midir.Opcode) bool {
	switch op {
	case midir.OpCall, midir.OpFnItem, midir.OpClosure, midir.OpBind:
		return true
	}
	return false
}
