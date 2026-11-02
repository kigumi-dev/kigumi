package borrow

import (
	"cmp"
	"slices"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

// checkBorrowedMoves rejects moving a borrowed value out of the function
// that borrows it: the caller keeps its value, so handing it on would alias
// two owners. A borrow is a `self` receiver or a `&T` local, and anything
// projected out of one (a field, an element, a payload) stays borrowed.
func checkBorrowedMoves(state *Analyzer, f *midir.Func) {
	p := state.Input
	checkCapturedFieldMoves(p, f)
	unowned := state.unownedRefs(f)
	borrowed := make([]bool, len(f.Locals))
	defNode := make([]syntax.NodeID, len(f.Locals))
	for i, l := range f.Locals {
		// A reference local that took its own count isn't aliasing the caller's.
		borrowed[i] = l.Borrowed || unowned[i]
	}
	isClosure := p.R.Entity(f.Ent).Kind == sem.EntClosure
	if isClosure {
		for i, cap := range p.R.Closure(f.Ent).Captures {
			if cap.Mode == sem.CapBorrow || cap.Mode == sem.CapRead {
				borrowed[f.Params[i]] = true
			}
		}
	}
	fieldRead := make([]syntax.NodeID, len(f.Locals))
	// origin is where a borrowed value was read, for a join temporary whose own copies carry no node.
	origin := make([]syntax.NodeID, len(f.Locals))
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() {
				defNode[in.Dst] = in.Node
				if in.Op == midir.OpField {
					fieldRead[in.Dst] = in.Node
				}
			}
			propagates := len(in.Args) > 0 && borrowed[in.Args[0]] && !in.Share && f.Locals[in.Dst].Ent == 0
			switch in.Op {
			case midir.OpField, midir.OpIndex, midir.OpPayload, midir.OpUnbox, midir.OpCopy:
				if propagates {
					borrowed[in.Dst] = true
					if origin[in.Dst] == 0 {
						origin[in.Dst] = cmp.Or(in.Node, origin[in.Args[0]])
					}
				}
			// A field taken out of a receiver is meant to escape its own
			// method; a closure only lends a captured self, so it stays tainted.
			case midir.OpFieldMove:
				if propagates && isClosure {
					borrowed[in.Dst] = true
				}
			case midir.OpCellGet:
				if propagates && f.Locals[in.Args[0]].Ent != 0 {
					borrowed[in.Dst] = true
				}
			}
		}
	}
	reported := map[syntax.NodeID]bool{}
	report := func(n syntax.NodeID, l midir.LocalID) {
		if n == 0 || reported[n] {
			return
		}
		reported[n] = true
		t := p.R.Tree(f.File)
		clone := "; clone it"
		if p.R.HoldsResource(f.Locals[l].Type) {
			clone = ""
		}
		msg := "a value borrowed here cannot be moved out" + clone
		if name := f.Locals[l].Name; f.Locals[l].Ent != 0 {
			msg = "`" + name + "` is borrowed by this method and cannot be moved out; clone it or take `move self`"
			if clone == "" {
				msg = "`" + name + "` is borrowed by this method and cannot be moved out; take `move self`"
			}
			if p.R.Entity(f.Ent).Kind == sem.EntClosure {
				msg = "`" + name + "` is lent to this closure and cannot be moved out; write `move` before the closure to give it `" + name + "`" + strings.Replace(clone, "; clone", ", or clone", 1)
			}
		}
		span := t.Span(n)
		f.Diags = slices.DeleteFunc(f.Diags, func(d diag.Diagnostic) bool {
			return d.Code == "E1025" && d.Loc.Span.Start >= span.Start && d.Loc.Span.End <= span.End
		})
		f.Diags = append(f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, span), Code: "move-from-borrow", Msg: msg})
	}
	mutRef := func(l midir.LocalID) bool {
		t := f.Locals[l].Type
		return p.R.Types.Kind(t) == sem.KRef && !p.R.IsCopy(t)
	}
	// checkAliasedMutArgs rejects a reborrowed `&mut T` (own_borrowed no
	// longer moves it) handed to the same call twice: the callee
	// would see two exclusive borrows of the one referent.
	checkAliasedMutArgs := func(n syntax.NodeID, args []midir.LocalID) {
		if n == 0 || reported[n] {
			return
		}
		seen := map[midir.LocalID]bool{}
		for _, a := range args {
			if !mutRef(a) {
				continue
			}
			if seen[a] {
				reported[n] = true
				t := p.R.Tree(f.File)
				f.Diags = append(f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Code: ownquery.DiagnosticCode("borrow-conflict"),
					Msg: "`" + f.Locals[a].Name + "` is passed twice as `&mut`; the callee would get two exclusive borrows of the same value"})
				return
			}
			seen[a] = true
		}
	}
	moveOnly := func(l midir.LocalID) bool {
		t := f.Locals[l].Type
		return borrowed[l] && !p.R.IsCopy(t) && p.R.Types.Kind(t) != sem.KRef
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			switch in.Op {
			case midir.OpMove:
				// refFieldMove's move out of `mut self` is a partial move, as OpFieldMove is.
				if borrowed[in.Args[0]] && (isClosure || in.Node == 0 || fieldRead[in.Args[0]] != in.Node) {
					report(in.Node, in.Args[0])
				}
			case midir.OpCopy:
				if f.Locals[in.Dst].Ent != 0 && !in.Share && moveOnly(in.Args[0]) {
					report(in.Node, in.Args[0])
				}
			case midir.OpSetField, midir.OpSetIndex, midir.OpNewCell:
				if v := in.Args[len(in.Args)-1]; moveOnly(v) && !in.Lent {
					report(in.Node, v)
				}
			case midir.OpCall, midir.OpCallValue, midir.OpBuiltin, midir.OpFutureNew:
				for i, a := range in.Args {
					// A runtime primitive has no body to lend from; its declaration says what it takes.
					if moveOnly(a) && (ownquery.ConsumesArg(p.R, p.ByEnt, &in, i) || ownquery.PrimitiveTakes(p.R, &in, i)) {
						report(in.Node, a)
					}
				}
				checkAliasedMutArgs(in.Node, in.Args)
			case midir.OpRecord, midir.OpVariant:
				for _, a := range in.Args {
					// A spread's projections were already reported at the spread itself.
					if moveOnly(a) && !reported[defNode[a]] {
						report(in.Node, a)
					}
				}
			}
		}
		if blk.Term.Op == midir.TermReturn {
			for _, a := range blk.Term.Args {
				if moveOnly(a) {
					report(cmp.Or(defNode[a], origin[a]), a)
				}
			}
		}
	}
}
