package plan

import (
	"slices"

	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// binder is where a body finds each parameter's runtime type: its environment, or a local a type test or a
// capture fills.
type binder struct {
	r      *sem.Result
	f      *midir.Func
	index  map[sem.EntityID]int
	opened map[sem.EntityID]midir.LocalID
	// params are the environment's arguments read once at entry, entry the reads.
	params map[sem.EntityID]midir.LocalID
	entry  []midir.Inst
}

func (ps *envPass) binder(f *midir.Func) *binder {
	b := &binder{r: ps.p.R, f: f, opened: openedIn(f), params: map[sem.EntityID]midir.LocalID{}}
	for p, l := range ps.captured[f] {
		b.opened[p] = l
	}
	if f.HasEnv {
		b.index = instance.EnvIndex(ps.p.R, f.EnvType)
	}
	return b
}

// value is the local holding p's runtime type, or for an opened const parameter its value.
func (b *binder) value(p sem.EntityID) ([]midir.Inst, midir.LocalID, bool) {
	if l, ok := b.opened[p]; ok {
		return nil, l, true
	}
	if l, ok := b.params[p]; ok {
		return nil, l, true
	}
	i, ok := b.index[p]
	switch {
	case !ok:
		return nil, 0, false
	case i == instance.WholeEnv:
		return nil, b.f.Env, true
	}
	in := midir.Inst{Op: midir.OpBuiltin, Str: "type.param", Dst: newTypeLocal(b.r, b.f, "$rt"), Index: i, Args: []midir.LocalID{b.f.Env}}
	b.params[p], b.entry = in.Dst, append(b.entry, in)
	return nil, in.Dst, true
}

// typeOf emits the runtime type t stands for in b's body; false when a parameter it names has no runtime value.
func (ps *envPass) typeOf(b *binder, t sem.TypeID, node midir.Inst) ([]midir.Inst, midir.LocalID, bool) {
	r := ps.p.R
	t = ps.p.Canon.Of(r, t)
	if b.f.HasEnv && t == b.f.EnvType {
		return nil, b.f.Env, true
	}
	in := midir.Inst{Op: midir.OpBuiltin, Str: "type.of", Type: t, Node: node.Node}
	var pre []midir.Inst
	for _, p := range instance.Params(r, t) {
		at, l, ok := b.value(p)
		if !ok {
			return nil, 0, false
		}
		pre = append(pre, at...)
		in.Holes, in.Args = append(in.Holes, p), append(in.Args, l)
	}
	in.Dst = newTypeLocal(r, b.f, "$rt")
	return append(pre, in), in.Dst, true
}

// rewrite composes f's runtime types where its static ones name parameters, and passes the environments of the
// bodies it calls or makes values of.
func (ps *envPass) rewrite(f *midir.Func) {
	ps.witnessTypes(f)
	b := ps.binder(f)
	for bid := range f.Blocks {
		var out []midir.Inst
		for _, in := range f.Blocks[bid].Insts {
			out = append(out, ps.rewriteInst(b, in)...)
		}
		f.Blocks[bid].Insts = out
	}
	if len(b.entry) > 0 {
		f.Blocks[0].Insts = append(b.entry, f.Blocks[0].Insts...)
	}
	uncountTypes(ps.p.R, f)
}

func (ps *envPass) rewriteInst(b *binder, in midir.Inst) []midir.Inst {
	f, r := b.f, ps.p.R
	var site sem.TypeID
	switch in.Op {
	case midir.OpIsType:
		if instance.TypeTest(r, in.Type) == instance.TestExact {
			site = in.Type
		}
	case midir.OpBox, midir.OpLendBox:
		site = in.Type
	case midir.OpRecord:
		if ps.dropReadsInstance(in.Ent) {
			site = f.Locals[in.Dst].Type
		}
	case midir.OpCall:
		if r.IsErrorAs(in.Ent) {
			site, _ = r.Types.IsOption(f.Locals[in.Dst].Type)
		}
	case midir.OpFnItem, midir.OpBind:
		if t, ok := errorAsValueTarget(r, f, in); ok && !instance.Concrete(r, t) {
			pre, l, ok := ps.typeOf(b, t, in)
			if ok {
				in.Op, in.Args, in.Index = midir.OpBind, []midir.LocalID{l}, 0
			}
			return append(pre, in)
		}
	case midir.OpClosure:
		if m, ok := ps.minted[in.Ent]; ok && r.Entity(in.Ent).Kind == sem.EntClosure {
			return ps.captureEnv(b, in, m)
		}
	}
	if site != 0 && !instance.Concrete(r, site) {
		pre, l, ok := ps.typeOf(b, site, in)
		if ok {
			in.Rt = []midir.LocalID{l}
		}
		return append(pre, in)
	}
	if _, env, ok := ps.calleeEnv(f, in); ok {
		return ps.passEnv(b, in, env)
	}
	return []midir.Inst{in}
}

// passEnv hands the callee of in its environment env: as an argument of a call, bound into a function value,
// or bound beside a method value's receiver.
func (ps *envPass) passEnv(b *binder, in midir.Inst, env sem.TypeID) []midir.Inst {
	f, r := b.f, ps.p.R
	pre, l, ok := ps.typeOf(b, env, in)
	if !ok {
		return []midir.Inst{in}
	}
	callee := ps.p.ByEnt[in.Ent]
	if in.Op == midir.OpFutureNew {
		item, _ := soleDef(f, in.Args[0])
		callee = ps.p.ByEnt[item.Ent]
	}
	at := slices.Index(callee.Params, callee.Env)
	switch in.Op {
	case midir.OpCall:
		in.Args = slices.Insert(slices.Clone(in.Args), at, l)
	case midir.OpFutureNew:
		in.Args = slices.Insert(slices.Clone(in.Args), 1+at, l)
	case midir.OpFnItem:
		in.Op, in.Args, in.Index = midir.OpBind, []midir.LocalID{l}, hiddenCount(r, in.Ent)
	case midir.OpBind:
		in.Args = append([]midir.LocalID{l}, in.Args...)
	case midir.OpClosure:
		bound := midir.Inst{Op: midir.OpBind, Dst: newLocal(f, "$bound", r.Fn(in.Ent).Sig), Ent: in.Ent, Args: []midir.LocalID{l}, Async: in.Async, Node: in.Node}
		in.Str, in.Args = midir.FormApplied.Tag(), append([]midir.LocalID{bound.Dst}, in.Args...)
		// The closure takes its own reference to each capture, as plan-rc's would have for this temporary.
		return append(append(pre, bound, in), midir.Inst{Op: midir.OpRelease, Args: []midir.LocalID{bound.Dst}, Node: in.Node})
	}
	return append(pre, in)
}

// captureEnv builds the copy of a lambda that captures its runtime types, handing it the environment and the
// opened arguments it reads.
func (ps *envPass) captureEnv(b *binder, in midir.Inst, m mintedClosure) []midir.Inst {
	var pre []midir.Inst
	args := slices.Clone(in.Args)
	if m.env {
		at, l, ok := ps.lambdaEnv(b, in.Ent)
		if !ok {
			return []midir.Inst{in}
		}
		pre, args = at, append(args, l)
	}
	for _, p := range m.opened {
		at, l, ok := b.value(p)
		if !ok {
			return []midir.Inst{in}
		}
		pre = append(pre, at...)
		args = append(args, l)
	}
	in.Ent, in.Args = m.ent, args
	return append(pre, in)
}

// lambdaEnv is the environment lambda c captures where b builds it: b's own, or, in a body specialized for one
// instantiation of c's generic, that instantiation's.
func (ps *envPass) lambdaEnv(b *binder, c sem.EntityID) ([]midir.Inst, midir.LocalID, bool) {
	if b.f.HasEnv {
		return nil, b.f.Env, true
	}
	r := ps.p.R
	env, _ := instance.EnvType(r, instance.Enclosing(r, r.ClosureOrigin(c)))
	return ps.typeOf(b, ps.substOf(b.f)(env), midir.Inst{})
}

func newLocal(f *midir.Func, name string, t sem.TypeID) midir.LocalID {
	f.Locals = append(f.Locals, midir.Local{Name: name, Type: t})
	return midir.LocalID(len(f.Locals) - 1)
}
