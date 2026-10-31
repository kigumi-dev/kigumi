package llgen

import (
	"cmp"
	"slices"
	"strings"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// boxRecord is what lowering one function did with scalar boxes: made holds the sites that created one (arg -1 for
// the instruction's result), handed the call arguments passed over in a box.
type boxRecord struct {
	made, handed map[boxSite]bool
	charged      map[siteAt]bool
	primitives   map[siteAt]opdef.StdOpId
	arg          int
}

type boxSite struct {
	at  siteAt
	arg int
}

var freshScalarBox = map[string]bool{rtInt.Name: true, rtFloat.Name: true, rtChar.Name: true, rtBoxInt128.Name: true, rtBinop.Name: true, rtUnop.Name: true, rtCast.Name: true, rtPtr.Name: true}

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

// ScalarBoxSites reports the argument and pointer-result boxes the native lowering creates.
func ScalarBoxSites(p *mir.Program, opts EmitOptions) mir.NativeBoxSites {
	e := prepareEmitter(p, opts)
	defer func() { e.boxes = nil }()
	out := mir.NativeBoxSites{Arguments: map[*mir.Func][]mir.ArgBox{}, Results: map[*mir.Func][]mir.ResultBox{}, Operations: map[*mir.Func][]mir.OperationBox{}}
	for _, f := range p.Funcs {
		if !e.keeps(f) || e.naked(f) {
			continue
		}
		e.boxes = &boxRecord{made: map[boxSite]bool{}, handed: map[boxSite]bool{}, charged: map[siteAt]bool{}, primitives: map[siteAt]opdef.StdOpId{}, arg: -1}
		if f.Async {
			e.asyncFunction(f)
		} else {
			e.function(f)
		}
		if boxes := e.boxes.argBoxes(e, f); len(boxes) > 0 {
			out.Arguments[f] = boxes
		}
		for bid, blk := range f.Blocks {
			for pc, in := range blk.Insts {
				at := siteAt{mir.BlockID(bid), pc}
				if box, ok := e.primitiveScalarBox(f, in, at); ok {
					out.Operations[f] = append(out.Operations[f], box)
				}
				if !in.Op.Yields() || !e.boxes.made[boxSite{at, -1}] || e.boxes.charged[at] {
					continue
				}
				t := f.Locals[in.Dst].Type
				if e.r.Types.Kind(t) == sem.KPtr || e.r.Types.IsCFn(t) {
					out.Results[f] = append(out.Results[f], mir.ResultBox{Block: mir.BlockID(bid), Inst: pc})
				}
			}
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
	made := false
	for _, at := range defs[l] {
		if r.made[boxSite{at, -1}] {
			r.charged[at] = true
			made = true
			continue
		}
		in := f.Blocks[at.blk].Insts[at.pc]
		switch in.Op {
		case mir.OpAlias, mir.OpShare, mir.OpMove, mir.OpCopy:
			if !seen[in.Args[0]] && r.madeFor(f, defs, in.Args[0], seen) {
				made = true
			}
		}
	}
	return made
}
