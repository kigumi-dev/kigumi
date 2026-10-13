package mir

import "kigumi/internal/sem"

const (
	builtinDirectEq   = "direct.eq"
	builtinDirectHash = "direct.hash"
)

func protoBuiltin(in Inst) bool {
	return in.Op == OpBuiltin && (in.Str == builtinDirectEq || in.Str == builtinDirectHash)
}

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

func (p *Program) protoSubject(f *Func) (sem.TypeID, bool) {
	if len(f.Params) == 0 {
		return 0, false
	}
	t := f.Locals[f.Params[0]].Type
	if p.R.Types.Kind(t) == sem.KRef {
		t = p.R.Types.Node(t).Elem
	}
	return t, true
}

func (p *Program) intrinsify(f *Func, kind string, t sem.TypeID) {
	name, ret := builtinDirectEq, sem.TyBool
	if kind == "hash" {
		name, ret = builtinDirectHash, sem.TyU64
	}
	node, operands := f.Blocks[0].Insts[0].Node, 1
	if kind == "equals" {
		operands = 2
	}
	locals := make([]Local, 0, len(f.Params)+1)
	params := make([]LocalID, len(f.Params))
	for i, prm := range f.Params {
		l := f.Locals[prm]
		if i < operands {
			l.Type = t
		}
		l.Borrowed, l.Cell = true, false
		locals, params[i] = append(locals, l), LocalID(i)
	}
	res := LocalID(len(locals))
	f.Locals, f.Params = append(locals, Local{Name: "r", Type: ret}), params
	f.Blocks = []Block{{
		Insts: []Inst{{Op: OpBuiltin, Str: name, Dst: res, Args: append([]LocalID(nil), params[:operands]...), Node: node}},
		Term:  Term{Op: TermReturn, Args: []LocalID{res}},
	}}
	f.Ret, f.PanicDrop, f.rcBase, f.Proto = ret, nil, len(f.Locals), kind
}

// A borrow that only passes a local is dropped and the call becomes the builtin, which takes the value.
func (p *Program) inlineProtoCalls() {
	for _, f := range p.Funcs {
		var uses map[LocalID]int
		var defs map[LocalID]*Inst
		for bid := range f.Blocks {
			for i := range f.Blocks[bid].Insts {
				in := &f.Blocks[bid].Insts[i]
				g := p.ByEnt[in.Ent]
				if in.Op != OpCall || g == nil || g.Proto == "" {
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

func (p *Program) inlineProtoCall(f *Func, in *Inst, g *Func, uses map[LocalID]int, defs map[LocalID]*Inst) {
	name, operands := builtinDirectEq, 2
	if g.Proto == "hash" {
		name, operands = builtinDirectHash, 1
	}
	if len(in.Args) < operands {
		return
	}
	args := append([]LocalID(nil), in.Args[:operands]...)
	var gone []*Inst
	for k, a := range args {
		want := g.Locals[g.Params[k]].Type
		if f.Locals[a].Type == want && !f.Locals[a].Cell {
			continue
		}
		d := defs[a]
		if d == nil || d.Op != OpBorrow || uses[a] != 1 || f.Locals[d.Args[0]].Type != want || f.Locals[d.Args[0]].Cell {
			return
		}
		args[k] = d.Args[0]
		gone = append(gone, d)
	}
	for _, a := range in.Args[operands:] {
		d := defs[a]
		if d == nil || uses[a] != 1 || (d.Op != OpBind && d.Op != OpFnItem && d.Op != OpClosure) {
			return
		}
		gone = append(gone, d)
	}
	for _, d := range gone {
		d.Op, d.Args, d.Dst = OpNop, nil, 0
	}
	*in = Inst{Op: OpBuiltin, Str: name, Dst: in.Dst, Args: args, Node: in.Node}
}

func useCounts(f *Func) map[LocalID]int {
	out := map[LocalID]int{}
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

func (p *Program) protoShape(f *Func, in Inst) string {
	kind, want, ret := "equals", 2, sem.TyBool
	if in.Str == builtinDirectHash {
		kind, want, ret = "hash", 1, sem.TyU64
	}
	if len(in.Args) != want || f.Locals[in.Dst].Type != ret {
		return "takes the wrong operands or yields the wrong type"
	}
	t := f.Locals[in.Args[0]].Type
	for _, a := range in.Args {
		if f.Locals[a].Type != t {
			return "compares or hashes operands of different types"
		}
	}
	if !p.R.ProtocolDirect(t, kind) {
		return "compares or hashes a type that is not direct"
	}
	return ""
}
