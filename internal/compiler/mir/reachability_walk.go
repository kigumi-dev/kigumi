package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

type reachWalk struct {
	p         *Program
	lower     func(sem.EntityID)
	lowerable func(sem.EntityID) bool
	r         *sem.Result
	rs        *Reachability
	worklist  []sem.EntityID
	seenTypes map[sem.TypeID]bool
	opSeen    map[opdef.StdOpId]bool
}

func (w *reachWalk) root(id sem.EntityID, kind RootKind) {
	if w.rs.add(id, Reason{Root: kind}) {
		w.worklist = append(w.worklist, id)
	}
}

func (w *reachWalk) markType(id sem.EntityID) {
	if id == 0 || w.r.Entity(id).Kind != sem.EntType || w.rs.Types[id] {
		return
	}
	w.rs.Types[id] = true
	w.root(w.r.TypeDecl(id).Drop, RootDropGlue)
}

// rt_error_as walks Context values by descriptor, so std/error.as keeps the type even when nothing builds one.
func (w *reachWalk) keepErrorContext(ent sem.EntityID) {
	if ent == 0 || w.r.Entity(ent).Kind != sem.EntFn || !w.r.Packages[w.r.Entity(ent).Pkg].Std || StdKey(w.r, ent, false) != "std/error.as" {
		return
	}
	w.markType(w.r.LangItem("Context"))
}

func (w *reachWalk) walkType(t sem.TypeID) {
	if t == 0 || w.seenTypes[t] {
		return
	}
	w.seenTypes[t] = true
	n := w.r.Types.Node(t)
	if n.Kind == sem.KNamed {
		w.markType(n.Ent)
	}
	w.walkType(n.Elem)
	for _, a := range n.Args {
		w.walkType(a)
	}
}

func (w *reachWalk) noteOp(op opdef.StdOpId, caller sem.EntityID, node syntax.NodeID) {
	w.rs.AllOpSites[op] = append(w.rs.AllOpSites[op], OpSite{Caller: caller, Node: node})
	if w.opSeen[op] {
		return
	}
	w.opSeen[op] = true
	w.rs.Ops = append(w.rs.Ops, op)
	w.rs.OpSites[op] = OpSite{Caller: caller, Node: node}
	for _, ref := range opdef.NativeTypes(op) {
		w.markType(w.r.PackageMember(w.r.PackageByPath(ref.Package), ref.Name))
	}
}

// A function value naming a bodiless std function still reaches its op.
func (w *reachWalk) mentionedOp(ent sem.EntityID) (opdef.StdOpId, bool) {
	if ent == 0 || w.hasBody(ent) {
		return 0, false
	}
	e := w.r.Entity(ent)
	if e.Kind != sem.EntFn || !w.r.Packages[e.Pkg].Std {
		return 0, false
	}
	return opdef.StdOpFor(StdKey(w.r, ent, true))
}

func (w *reachWalk) visit(caller, target sem.EntityID, via string) {
	if target == 0 || !w.hasBody(target) {
		return
	}
	if w.rs.add(target, Reason{Via: via, Caller: caller}) {
		w.worklist = append(w.worklist, target)
	}
}

func (w *reachWalk) stdOp(in *midir.Inst) (opdef.StdOpId, bool) {
	if w.lower != nil {
		return stdOpOf(w.r, in)
	}
	return in.StdOp()
}

func (w *reachWalk) hasBody(ent sem.EntityID) bool {
	return w.p.ByEnt[ent] != nil || w.lowerable != nil && w.lowerable(ent)
}

// inst follows one instruction of f; pkg is the package f's code belongs to.
func (w *reachWalk) inst(f *midir.Func, id sem.EntityID, pkg sem.PackageID, in *midir.Inst) {
	w.visit(id, in.Ent, "call")
	w.keepErrorContext(in.Ent)
	w.walkType(in.Type)
	switch in.Op {
	case midir.OpRecord, midir.OpIsType:
		w.markType(in.Ent)
	case midir.OpVariant, midir.OpIsVariant:
		if parent := w.r.Entity(in.Ent).Parent; parent != 0 {
			w.markType(parent)
		}
	}
	if in.Op == midir.OpShell {
		w.visit(id, shellRuntimeFn(w.r, "planNew"), "call")
		w.visit(id, shellRuntimeFn(w.r, "planPush"), "call")
	}
	if in.Op == midir.OpBox || in.Op == midir.OpLendBox {
		dstT := f.Locals[in.Dst].Type
		if in.Op == midir.OpLendBox {
			dstT = w.r.Types.Node(dstT).Elem
		}
		for _, wit := range w.r.Witnesses(in.Type, dstT, pkg) {
			w.visit(id, wit, "witness")
		}
		if w.r.Types.Kind(in.Type) != sem.KNamed {
			w.markType(w.r.EntityOfName(w.r.TypeString(in.Type)))
		}
	}
	if op, ok := w.stdOp(in); ok {
		w.noteOp(op, id, in.Node)
	} else if op, ok := w.mentionedOp(in.Ent); ok {
		w.noteOp(op, id, in.Node)
	}
}

func newReachWalk(p *Program) *reachWalk {
	w := &reachWalk{p: p, r: p.R, seenTypes: map[sem.TypeID]bool{}, opSeen: map[opdef.StdOpId]bool{}}
	w.rs = &Reachability{Funcs: map[sem.EntityID]bool{}, Reason: map[sem.EntityID]Reason{}, OpSites: map[opdef.StdOpId]OpSite{}, AllOpSites: map[opdef.StdOpId][]OpSite{}, Types: map[sem.EntityID]bool{}}
	return w
}

func Reachable(p *Program) *Reachability {
	w := newReachWalk(p)
	if p.Entry != nil {
		w.root(p.Entry.Ent, RootEntry)
	}
	for _, f := range p.Funcs {
		ent := w.r.Entity(f.Ent)
		switch {
		case ent.Kind == sem.EntTest:
			w.root(f.Ent, RootTest)
		case ent.Kind == sem.EntFn && w.r.Fn(f.Ent).Export != "":
			w.root(f.Ent, RootExport)
		}
	}
	w.drain()
	return w.rs
}

func (w *reachWalk) drain() {
	for len(w.worklist) > 0 {
		id := w.worklist[len(w.worklist)-1]
		w.worklist = w.worklist[:len(w.worklist)-1]
		if w.lower != nil {
			w.lower(id)
		}
		if f := w.p.ByEnt[id]; f != nil {
			w.scan(f, id, w.r.Entity(id).Pkg)
		}
	}
}

func (w *reachWalk) scan(f *midir.Func, id sem.EntityID, pkg sem.PackageID) {
	w.walkType(f.Ret)
	for _, l := range f.Locals {
		w.walkType(l.Type)
	}
	for _, blk := range f.Blocks {
		for i := range blk.Insts {
			w.inst(f, id, pkg, &blk.Insts[i])
		}
	}
}
