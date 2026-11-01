package verify

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type variantFlow struct {
	weak    []bool
	p       *verifier
	f       *midir.Func
	cond    []bool
	lenOf   map[midir.LocalID]midir.LocalID
	carried []bool
	multi   []bool
	live    *edgeLive
	// inert marks locals whose values never carry variant, cell or ordering facts, so an alias of one is not tracked.
	inert []bool
}

func (p *verifier) verifyVariants(f *midir.Func) *Error {
	if !p.needsFacts(f) {
		return nil
	}
	cond := condLocals(f)
	vf := &variantFlow{p: p, f: f, weak: p.body(f).weakDefs, cond: cond, lenOf: iterLengths(f), carried: carriedBools(f, cond), multi: multiDefs(f), live: p.edgeLive(f), inert: p.inertLocals(f)}
	in := make([]*variantFacts, len(f.Blocks))
	in[0] = newFacts(len(f.Locals))
	var found *Error
	acyclic := solveForward(f, func(bid midir.BlockID, wake func(midir.BlockID), final bool) bool {
		out, err := vf.block(bid, in[bid], final)
		if err != nil {
			found = err
			return false
		}
		if diverges(f, &f.Blocks[bid]) {
			return true
		}
		term := f.Blocks[bid].Term
		for k, succ := range term.Targets {
			edge := out
			if k < len(term.Targets)-1 {
				edge = out.clone()
			}
			if term.Op == midir.TermBranch && !edge.assume(term.Args[0], k == 0) {
				continue
			}
			vf.live.dead(f, bid, edge.retire)
			edge.keepOnly(vf.carried)
			edge.fold()
			switch {
			case in[succ] == nil:
				in[succ] = edge
				wake(succ)
			case in[succ].join(edge):
				wake(succ)
			}
		}
		return true
	})
	if found != nil || acyclic {
		return found
	}
	for bid := range f.Blocks {
		if in[bid] == nil {
			continue
		}
		if _, err := vf.block(midir.BlockID(bid), in[bid], true); err != nil {
			return err
		}
	}
	return nil
}

func (p *verifier) needsFacts(f *midir.Func) bool {
	if slices.Contains(p.body(f).weakDefs, true) {
		return true
	}
	if cellReadAfterTake(f) {
		return true
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpBuiltin && iterAt(in.Str) {
				return true
			}
			if in.Op != midir.OpPayload {
				continue
			}
			kind, t := p.aggOf(f.Locals[in.Args[0]].Type)
			if kind != aggAdt {
				continue
			}
			for _, v := range p.R.AggVariants(t) {
				if in.Index >= len(p.R.Variant(v).Payload) {
					return true
				}
			}
		}
	}
	return false
}

func condLocals(f *midir.Func) []bool {
	cond := make([]bool, len(f.Locals))
	for _, blk := range f.Blocks {
		if blk.Term.Op == midir.TermBranch {
			cond[blk.Term.Args[0]] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op.Yields() && cond[in.Dst] && copiesBool(in) && !cond[in.Args[0]] {
					cond[in.Args[0]], changed = true, true
				}
			}
		}
	}
	return cond
}

func copiesBool(in midir.Inst) bool {
	switch in.Op {
	case midir.OpAlias, midir.OpCopy, midir.OpMove, midir.OpShare:
		return len(in.Args) == 1 && in.Args[0] != in.Dst
	case midir.OpUnary:
		return in.Str == "!" && in.Args[0] != in.Dst
	}
	return false
}

// A place named through a local that several instructions define would change root between passes of the solver.
func multiDefs(f *midir.Func) []bool {
	count := make([]int, len(f.Locals))
	for _, prm := range f.Params {
		count[prm]++
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() {
				count[in.Dst]++
			}
		}
	}
	multi := make([]bool, len(count))
	for l, n := range count {
		multi[l] = n > 1
	}
	return multi
}

func (p *verifier) inertLocals(f *midir.Func) []bool {
	inert := make([]bool, len(f.Locals))
	for i, l := range f.Locals {
		inert[i] = !l.Cell && p.opaqueValue(l.Type)
	}
	return inert
}

func (p *verifier) opaqueValue(t sem.TypeID) bool {
	t = p.R.UnderlyingRepr(p.DerefAll(t))
	if t == sem.TyString {
		return true
	}
	return p.R.Types.Kind(t) == sem.KNamed && p.R.TypeDecl(p.R.Types.Node(t).Ent).Form == sem.FormOpaque
}
