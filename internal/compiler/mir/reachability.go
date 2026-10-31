package mir

import (
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
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
	Ops     []opdef.StdOpId
	OpSites map[opdef.StdOpId]OpSite
	// AllOpSites lists every reachable call site per op, in discovery order; OpSites holds the first.
	AllOpSites map[opdef.StdOpId][]OpSite
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
