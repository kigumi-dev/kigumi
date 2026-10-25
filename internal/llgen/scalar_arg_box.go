package llgen

import (
	"cmp"
	"slices"
	"strings"

	"kigumi/internal/mir"
)

// boxRecord is what lowering one function did with scalar boxes: made holds the sites that created one (arg -1 for
// the instruction's result), handed the call arguments passed over in a box.
type boxRecord struct {
	made, handed map[boxSite]bool
	arg          int
}

type boxSite struct {
	at  siteAt
	arg int
}

var freshScalarBox = map[string]bool{rtInt.Name: true, rtFloat.Name: true, rtChar.Name: true, rtBoxInt128.Name: true, rtBinop.Name: true, rtUnop.Name: true, rtCast.Name: true}

func (e *emitter) noteRuntimeCall(fn RuntimeFn) {
	if freshScalarBox[fn.Name] {
		e.boxes.made[boxSite{e.at, e.boxes.arg}] = true
	}
}

func (e *emitter) noteHanded(i int) {
	if e.boxes != nil {
		e.boxes.handed[boxSite{e.at, i}] = true
	}
}

func (e *emitter) argBoxOnDemand(sb *strings.Builder, i int, l mir.LocalID) string {
	if e.boxes == nil {
		return e.scalarBoxOnDemand(sb, l)
	}
	e.boxes.arg = i
	defer func() { e.boxes.arg = -1 }()
	return e.scalarBoxOnDemand(sb, l)
}

// ScalarArgBoxes lowers every function EmitProgram would under opts and lists the call arguments whose box that
// lowering created in the caller, at the call or where the value was made.
func ScalarArgBoxes(p *mir.Program, opts EmitOptions) map[*mir.Func][]mir.ArgBox {
	e := prepareEmitter(p, opts)
	defer func() { e.boxes = nil }()
	out := map[*mir.Func][]mir.ArgBox{}
	for _, f := range p.Funcs {
		if !e.keeps(f) || e.naked(f) {
			continue
		}
		e.boxes = &boxRecord{made: map[boxSite]bool{}, handed: map[boxSite]bool{}, arg: -1}
		if f.Async {
			e.asyncFunction(f)
		} else {
			e.function(f)
		}
		if boxes := e.boxes.argBoxes(e, f); len(boxes) > 0 {
			out[f] = boxes
		}
	}
	return out
}

func (r *boxRecord) argBoxes(e *emitter, f *mir.Func) []mir.ArgBox {
	defs := map[mir.LocalID][]siteAt{}
	for bid, blk := range f.Blocks {
		for pc, in := range blk.Insts {
			if in.Op.Yields() {
				defs[in.Dst] = append(defs[in.Dst], siteAt{mir.BlockID(bid), pc})
			}
		}
	}
	var out []mir.ArgBox
	for s := range r.handed {
		a := f.Blocks[s.at.blk].Insts[s.at.pc].Args[s.arg]
		if e.r.ScalarBoxes(f.Locals[a].Type) && (r.made[s] || r.madeFor(f, defs, a, map[mir.LocalID]bool{})) {
			out = append(out, mir.ArgBox{Block: s.at.blk, Inst: s.at.pc, Arg: s.arg})
		}
	}
	slices.SortFunc(out, func(x, y mir.ArgBox) int {
		return cmp.Or(cmp.Compare(x.Block, y.Block), cmp.Compare(x.Inst, y.Inst), cmp.Compare(x.Arg, y.Arg))
	})
	return out
}

// madeFor follows l back through the copies that pass a box on unchanged to the instructions that created it.
func (r *boxRecord) madeFor(f *mir.Func, defs map[mir.LocalID][]siteAt, l mir.LocalID, seen map[mir.LocalID]bool) bool {
	seen[l] = true
	for _, at := range defs[l] {
		if r.made[boxSite{at, -1}] {
			return true
		}
		in := f.Blocks[at.blk].Insts[at.pc]
		switch in.Op {
		case mir.OpAlias, mir.OpShare, mir.OpMove, mir.OpCopy:
			if !seen[in.Args[0]] && r.madeFor(f, defs, in.Args[0], seen) {
				return true
			}
		}
	}
	return false
}
