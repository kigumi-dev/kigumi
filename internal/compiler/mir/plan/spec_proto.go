package plan

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (s *specializer) intrinsifyOwned() {
	r := s.p.R
	for _, f := range s.p.Funcs {
		kind := r.ProtoKind(f.Ent)
		if kind == "" || r.Fn(f.Ent).Owner == 0 || len(r.Fn(f.Ent).TypeParams) > 0 {
			continue
		}
		if t, ok := s.p.protoSubject(f); ok && r.ProtocolDirect(t, kind) {
			s.p.intrinsify(f, kind, t)
		}
	}
}

func (p *planner) protoSubject(f *midir.Func) (sem.TypeID, bool) {
	if len(f.Params) == 0 {
		return 0, false
	}
	t := f.Locals[f.Params[0]].Type
	if p.R.Types.Kind(t) == sem.KRef {
		t = p.R.Types.Node(t).Elem
	}
	return t, true
}

func (p *planner) intrinsify(f *midir.Func, kind string, t sem.TypeID) {
	name, ret := mir.BuiltinDirectEq, sem.TyBool
	if kind == "hash" {
		name, ret = mir.BuiltinDirectHash, sem.TyU64
	}
	node, operands := f.Blocks[0].Insts[0].Node, 1
	if kind == "equals" {
		operands = 2
	}
	locals := make([]midir.Local, 0, len(f.Params)+1)
	params := make([]midir.LocalID, len(f.Params))
	for i, prm := range f.Params {
		l := f.Locals[prm]
		if i < operands {
			l.Type = t
		}
		l.Cell = false
		if i < operands {
			l.Borrowed = true
		}
		locals, params[i] = append(locals, l), midir.LocalID(i)
	}
	res := midir.LocalID(len(locals))
	f.Locals, f.Params = append(locals, midir.Local{Name: "r", Type: ret}), params
	insts := []midir.Inst{{Op: midir.OpBuiltin, Str: name, Dst: res, Args: append([]midir.LocalID(nil), params[:operands]...), Node: node}}
	for _, prm := range params[operands:] {
		if !f.Locals[prm].Borrowed {
			insts = append(insts, midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{prm}, Node: node})
		}
	}
	f.Blocks = []midir.Block{{
		Insts: insts,
		Term:  midir.Term{Op: midir.TermReturn, Args: []midir.LocalID{res}},
	}}
	f.Ret, f.PanicDrop, f.RCBase, f.Proto = ret, nil, len(f.Locals), kind
}

// A borrow that only passes a local is dropped and the call becomes the builtin, which reads the value without consuming it.
func (p *planner) inlineProtoCalls() {
	for _, f := range p.Funcs {
		var uses map[midir.LocalID]int
		var defs map[midir.LocalID]*midir.Inst
		for bid := range f.Blocks {
			for i := range f.Blocks[bid].Insts {
				in := &f.Blocks[bid].Insts[i]
				g := p.ByEnt[in.Ent]
				if in.Op != midir.OpCall || g == nil || g.Proto == "" {
					continue
				}
				if uses == nil {
					uses, defs = useCounts(f), singleDefs(f)
				}
				p.inlineProtoCall(f, in, g, uses, defs)
			}
		}
	}
}

func (p *planner) inlineProtoCall(f *midir.Func, in *midir.Inst, g *midir.Func, uses map[midir.LocalID]int, defs map[midir.LocalID]*midir.Inst) {
	name, operands := mir.BuiltinDirectEq, 2
	if g.Proto == "hash" {
		name, operands = mir.BuiltinDirectHash, 1
	}
	if len(in.Args) < operands {
		return
	}
	args := append([]midir.LocalID(nil), in.Args[:operands]...)
	var gone []*midir.Inst
	for k, a := range args {
		want := g.Locals[g.Params[k]].Type
		if f.Locals[a].Type == want && !f.Locals[a].Cell {
			continue
		}
		d := defs[a]
		if d == nil || d.Op != midir.OpBorrow || uses[a] != 1 || f.Locals[d.Args[0]].Type != want || f.Locals[d.Args[0]].Cell {
			return
		}
		args[k] = d.Args[0]
		gone = append(gone, d)
	}
	for _, a := range in.Args[operands:] {
		d := defs[a]
		if d == nil || uses[a] != 1 || (d.Op != midir.OpBind && d.Op != midir.OpFnItem && d.Op != midir.OpClosure) {
			return
		}
		gone = append(gone, d)
	}
	for _, d := range gone {
		*d = midir.Inst{Op: midir.OpNop, Node: d.Node}
	}
	*in = midir.Inst{Op: midir.OpBuiltin, Str: name, Dst: in.Dst, Args: args, Node: in.Node}
}

func useCounts(f *midir.Func) map[midir.LocalID]int {
	out := map[midir.LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				out[a]++
			}
		}
		for _, a := range blk.Term.Args {
			out[a]++
		}
	}
	return out
}
