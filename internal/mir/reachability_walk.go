package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

type reachWalk struct {
	p         *Program
	r         *sem.Result
	rs        *Reachability
	worklist  []sem.EntityID
	seenTypes map[sem.TypeID]bool
	opSeen    map[StdOpId]bool
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

func (w *reachWalk) noteOp(op StdOpId, caller sem.EntityID, node syntax.NodeID) {
	w.rs.AllOpSites[op] = append(w.rs.AllOpSites[op], OpSite{Caller: caller, Node: node})
	if w.opSeen[op] {
		return
	}
	w.opSeen[op] = true
	w.rs.Ops = append(w.rs.Ops, op)
	w.rs.OpSites[op] = OpSite{Caller: caller, Node: node}
	for _, ref := range nativeRecordTypesForOp[op] {
		w.markType(w.r.PackageMember(w.r.PackageByPath(ref.pkg), ref.name))
	}
}

// A function value naming a bodiless std function still reaches its op.
func (w *reachWalk) mentionedOp(ent sem.EntityID) (StdOpId, bool) {
	if ent == 0 || w.p.ByEnt[ent] != nil {
		return 0, false
	}
	e := w.r.Entity(ent)
	if e.Kind != sem.EntFn || !w.r.Packages[e.Pkg].Std {
		return 0, false
	}
	return StdOpFor(StdKey(w.r, ent, true))
}

func (w *reachWalk) visit(caller, target sem.EntityID, via string) {
	if target == 0 || w.p.ByEnt[target] == nil {
		return
	}
	if w.rs.add(target, Reason{Via: via, Caller: caller}) {
		w.worklist = append(w.worklist, target)
	}
}

func (w *reachWalk) inst(f *Func, id sem.EntityID, in *Inst) {
	w.visit(id, in.Ent, "call")
	w.walkType(in.Type)
	switch in.Op {
	case OpRecord, OpIsType:
		w.markType(in.Ent)
	case OpVariant, OpIsVariant:
		if parent := w.r.Entity(in.Ent).Parent; parent != 0 {
			w.markType(parent)
		}
	}
	if in.Op == OpShell {
		w.visit(id, shellRuntimeFn(w.r, "planNew"), "call")
		w.visit(id, shellRuntimeFn(w.r, "planPush"), "call")
	}
	if in.Op == OpBox {
		dstT := f.Locals[in.Dst].Type
		for _, wit := range w.r.Witnesses(in.Type, dstT, w.r.Entity(f.Ent).Pkg) {
			w.visit(id, wit, "witness")
		}
	}
	if op, ok := in.StdOp(); ok {
		w.noteOp(op, id, in.Node)
	} else if op, ok := w.mentionedOp(in.Ent); ok {
		w.noteOp(op, id, in.Node)
	}
}

func Reachable(p *Program) *Reachability {
	w := &reachWalk{p: p, r: p.R, seenTypes: map[sem.TypeID]bool{}, opSeen: map[StdOpId]bool{}}
	w.rs = &Reachability{Funcs: map[sem.EntityID]bool{}, Reason: map[sem.EntityID]Reason{}, OpSites: map[StdOpId]OpSite{}, AllOpSites: map[StdOpId][]OpSite{}, Types: map[sem.EntityID]bool{}}
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
	for len(w.worklist) > 0 {
		id := w.worklist[len(w.worklist)-1]
		w.worklist = w.worklist[:len(w.worklist)-1]
		f := p.ByEnt[id]
		if f == nil {
			continue
		}
		w.walkType(f.Ret)
		for _, l := range f.Locals {
			w.walkType(l.Type)
		}
		for _, blk := range f.Blocks {
			for i := range blk.Insts {
				w.inst(f, id, &blk.Insts[i])
			}
		}
	}
	return w.rs
}
