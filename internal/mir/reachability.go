package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// Zero means the entity was reached by an edge from another kept entity, not a root.
type RootKind uint8

const (
	RootNone RootKind = iota
	RootEntry
	RootExport
	RootTest
	// A kept type's drop runs through its descriptor's function pointer
	// (rt_drop), never a direct call edge, so it must be rooted explicitly.
	RootDropGlue
)

// Root is set for a canonical root; otherwise Via/Caller name the edge that reached it.
type Reason struct {
	Root   RootKind
	Via    string // "call", "witness" for a non-root reason; "" for a root
	Caller sem.EntityID
}

// OpSite is the first reachable call site resolved to a given std op, for
// a diagnostic that names a path from a root.
type OpSite struct {
	Caller sem.EntityID
	Node   syntax.NodeID
}

// Reachability is the program-level reachability result that drives llgen
// emission and (for provider ops) the strict-profile capability check.
type Reachability struct {
	Funcs   map[sem.EntityID]bool
	Reason  map[sem.EntityID]Reason
	Order   []sem.EntityID
	Ops     []StdOpId
	OpSites map[StdOpId]OpSite
	// Types are the named types data() must give a real descriptor: built,
	// held or tested by reachable code, or built by name in rt_core.c.
	Types map[sem.EntityID]bool
}

func (rs *Reachability) Keep(ent sem.EntityID) bool { return rs.Funcs[ent] }

func (rs *Reachability) KeepType(ent sem.EntityID) bool { return rs.Types[ent] }

func (rs *Reachability) add(id sem.EntityID, reason Reason) bool {
	if id == 0 || rs.Funcs[id] {
		return false
	}
	rs.Funcs[id] = true
	rs.Reason[id] = reason
	rs.Order = append(rs.Order, id)
	return true
}

// Reachable computes p's canonical reachability set: entry, exported symbols and tests as
// roots, closed over call/value/bind edges, box witnesses, and the named types those reach.
func Reachable(p *Program) *Reachability {
	r := p.R
	rs := &Reachability{Funcs: map[sem.EntityID]bool{}, Reason: map[sem.EntityID]Reason{}, OpSites: map[StdOpId]OpSite{}, Types: map[sem.EntityID]bool{}}
	var worklist []sem.EntityID
	root := func(id sem.EntityID, kind RootKind) {
		if rs.add(id, Reason{Root: kind}) {
			worklist = append(worklist, id)
		}
	}
	markType := func(id sem.EntityID) {
		if id == 0 || r.Entity(id).Kind != sem.EntType || rs.Types[id] {
			return
		}
		rs.Types[id] = true
		root(r.TypeDecl(id).Drop, RootDropGlue)
	}
	seenTypeIDs := map[sem.TypeID]bool{}
	var walkType func(t sem.TypeID)
	walkType = func(t sem.TypeID) {
		if t == 0 || seenTypeIDs[t] {
			return
		}
		seenTypeIDs[t] = true
		n := r.Types.Node(t)
		if n.Kind == sem.KNamed {
			markType(n.Ent)
		}
		walkType(n.Elem)
		for _, a := range n.Args {
			walkType(a)
		}
	}
	for _, ent := range baseTypeEnts(r) {
		markType(ent)
	}
	if p.Entry != nil {
		root(p.Entry.Ent, RootEntry)
	}
	for _, f := range p.Funcs {
		ent := r.Entity(f.Ent)
		switch {
		case ent.Kind == sem.EntTest:
			root(f.Ent, RootTest)
		case ent.Kind == sem.EntFn && r.Fn(f.Ent).Export != "":
			root(f.Ent, RootExport)
		}
	}
	opSeen := map[StdOpId]bool{}
	for len(worklist) > 0 {
		id := worklist[len(worklist)-1]
		worklist = worklist[:len(worklist)-1]
		f := p.ByEnt[id]
		if f == nil {
			continue
		}
		visit := func(target sem.EntityID, via string) {
			if target == 0 || p.ByEnt[target] == nil {
				return
			}
			if rs.add(target, Reason{Via: via, Caller: id}) {
				worklist = append(worklist, target)
			}
		}
		walkType(f.Ret)
		for _, l := range f.Locals {
			walkType(l.Type)
		}
		for _, blk := range f.Blocks {
			for i := range blk.Insts {
				in := &blk.Insts[i]
				visit(in.Ent, "call")
				walkType(in.Type)
				switch in.Op {
				case OpRecord:
					markType(in.Ent)
				case OpVariant, OpIsVariant:
					if parent := r.Entity(in.Ent).Parent; parent != 0 {
						markType(parent)
					}
				case OpIsType:
					markType(in.Ent)
				}
				if in.Op == OpShell {
					visit(shellRuntimeFn(r, "planNew"), "call")
					visit(shellRuntimeFn(r, "planPush"), "call")
				}
				if in.Op == OpBox {
					dstT := f.Locals[in.Dst].Type
					for _, w := range r.Witnesses(in.Type, dstT, r.Entity(f.Ent).Pkg) {
						visit(w, "witness")
					}
				}
				if op, ok := in.StdOp(); ok && !opSeen[op] {
					opSeen[op] = true
					rs.Ops = append(rs.Ops, op)
					rs.OpSites[op] = OpSite{Caller: id, Node: in.Node}
					for _, ref := range nativeRecordTypesForOp[op] {
						markType(r.PackageMember(r.PackageByPath(ref.pkg), ref.name))
					}
				}
			}
		}
	}
	return rs
}

// shellRuntimeFn resolves a `$"..."` literal's hidden std/shell builder call;
// emit_const.go resolves the same pair by name, since OpShell has no entity.
func shellRuntimeFn(r *sem.Result, name string) sem.EntityID {
	return r.PackageMember(r.PackageByPath("std/shell"), name)
}

// Root first; seen guards a cycle that should never occur.
func (rs *Reachability) PathNames(r *sem.Result, ent sem.EntityID) []string {
	var chain []string
	seen := map[sem.EntityID]bool{}
	for ent != 0 && !seen[ent] {
		seen[ent] = true
		chain = append(chain, r.Entity(ent).Name)
		reason, ok := rs.Reason[ent]
		if !ok || reason.Root != RootNone {
			break
		}
		ent = reason.Caller
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}
