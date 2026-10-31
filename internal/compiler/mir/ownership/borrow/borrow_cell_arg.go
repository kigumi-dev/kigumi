package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// argRunnable reports whether in may run a closure handed to it as argument i: the callee of a value call, an
// awaited future, or a parameter whose declared type can hold something callable or awaitable of a fn that may run.
func (c *borrowCheck) argRunnable(in midir.Inst, i int) bool {
	switch in.Op {
	case midir.OpAwait:
		return true
	case midir.OpCallValue:
		if i == 0 {
			return true
		}
		t, known := c.callParam(c.f, in, i)
		return !known || c.typeRuns(t, runsCall)
	case midir.OpCall:
		if in.Ent == 0 || c.p.R.Entity(in.Ent).Kind != sem.EntFn {
			return true
		}
		t, known := c.callParam(c.f, in, i)
		return (!known || c.typeRuns(t, runsCall)) && c.mayRun(in.Ent)
	}
	return false
}

// runEdges is whether ent itself may run a value it did not name, and what it calls.
func (c *borrowCheck) runEdges(ent sem.EntityID) (bool, []sem.EntityID) {
	var edges []sem.EffectEdge
	switch c.p.R.Entity(ent).Kind {
	case sem.EntClosure:
		edges = c.p.R.Closure(ent).Edges
	case sem.EntFn:
		info := c.p.R.Fn(ent)
		if info.Body == 0 {
			return info.SelfParam != 0 && c.typeRuns(c.p.R.Entity(info.SelfParam).Type, runsCall) ||
				slices.ContainsFunc(info.Params, func(p sem.EntityID) bool { return c.typeRuns(c.p.R.Entity(p).Type, runsCall) }), nil
		}
		edges = info.Edges
	default:
		return true, nil
	}
	var out []sem.EntityID
	for _, e := range edges {
		switch e.Kind {
		case sem.EdgeCall, sem.EdgeKnownCall:
			if e.Target == 0 {
				return true, nil
			}
			out = append(out, e.Target)
		case sem.EdgeDrop:
			if c.typeRuns(e.Type, runsDrop) {
				return true, nil
			}
		case sem.EdgeIO, sem.EdgeMutateCaller, sem.EdgeAlloc, sem.EdgeContract, sem.EdgePanic, sem.EdgeIfaceArg, sem.EdgeFieldBind, sem.EdgeFnBoundArg:
		default:
			return true, nil
		}
	}
	return false, out
}

type typeRunKey struct {
	t    sem.TypeID
	kind uint8
}

const (
	runsCall uint8 = iota
	runsSpawn
	runsDrop
	runsCallable
	runsHandle
	runsShared
)

// typeRuns memoizes a typeHolds question about t: runsCall (mayRunType), runsSpawn (a Future or Task), runsDrop (a user
// `drop` may run), runsCallable (a callableNode), runsHandle (holdsHandle) or runsShared (a sharedNode).
func (c *borrowCheck) typeRuns(t sem.TypeID, kind uint8) bool {
	key := typeRunKey{t, kind}
	if v, ok := c.typeRunMemo[key]; ok {
		return v
	}
	if c.typeRunMemo == nil {
		c.typeRunMemo = map[typeRunKey]bool{}
	}
	var v bool
	switch kind {
	case runsCall:
		v = c.mayRunType(t)
	case runsSpawn:
		v = c.holdsAsync(t)
	case runsCallable:
		v = c.typeHolds(t, c.callableNode)
	case runsHandle:
		v = c.holdsHandle(t)
	case runsShared:
		v = c.typeHoldsUntil(t, c.sharedNode, func(t sem.TypeID) bool { return c.p.R.Types.Kind(t) == sem.KFn })
	case runsDrop:
		c.typeRunMemo[key] = false
		v = c.typeHolds(t, func(t sem.TypeID) bool {
			n := c.p.R.Types.Node(t)
			if n.Kind == sem.KIface {
				return true
			}
			if n.Kind == sem.KClosure && n.Ent != 0 && c.p.R.Entity(n.Ent).Kind == sem.EntClosure {
				for _, cp := range c.p.R.Closure(n.Ent).Captures {
					if c.typeRuns(c.p.R.Entity(cp.Local).Type, runsDrop) {
						return true
					}
				}
				return false
			}
			if n.Kind != sem.KNamed || c.p.R.Entity(n.Ent).Kind != sem.EntType {
				return false
			}
			d := c.p.R.TypeDecl(n.Ent)
			return d != nil && d.Form == sem.FormResource && d.Drop != 0
		})
	}
	c.typeRunMemo[key] = v
	return v
}

// mayRunType reports whether a parameter declared t can reach a value to call or await: a fn, closure, interface or
// Future / Task inside it, or a type parameter bound to be callable; an unbounded `T` can only be moved around.
func (c *borrowCheck) mayRunType(t sem.TypeID) bool {
	return c.typeHolds(t, func(t sem.TypeID) bool {
		n := c.p.R.Types.Node(t)
		return c.callableNode(t) || n.Kind == sem.KNamed && n.Ent == c.p.R.Types.FutureEnt() || c.p.R.IsTaskType(t)
	})
}

// callableNode reports a fn, closure, interface or open type, or a type parameter bound to be callable.
func (c *borrowCheck) callableNode(t sem.TypeID) bool {
	n := c.p.R.Types.Node(t)
	switch n.Kind {
	case sem.KFn, sem.KClosure, sem.KIface, sem.KVar:
		return true
	case sem.KParam:
		return slices.ContainsFunc(c.p.R.TypeParam(n.Ent).Constraints, func(con sem.Constraint) bool {
			return con.Kind == sem.CFn || con.Kind == sem.CFnMut || con.Kind == sem.CFnOnce || con.Kind == sem.CIface
		})
	}
	return false
}
