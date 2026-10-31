package mir

import (
	"fmt"
	"slices"

	"kigumi/internal/compiler/sem"
)

type viewSourceSet struct {
	locals   []LocalID
	complete bool
}

type viewHop struct {
	local LocalID
	path  []int
}

func (c *borrowCheck) viewType(t sem.TypeID) bool {
	for c.p.R.Types.Kind(t) == sem.KRef {
		t = c.p.R.Types.Node(t).Elem
	}
	return c.p.R.Types.Kind(t) == sem.KIface
}

func (c *borrowCheck) viewSources(a LocalID) viewSourceSet {
	if !c.viewType(c.f.Locals[a].Type) {
		return viewSourceSet{locals: []LocalID{a}, complete: true}
	}
	if s, ok := c.viewSets[a]; ok {
		return s
	}
	if c.viewDefs == nil {
		c.viewDefs = allDefs(c.f)
		for bi := range c.f.Blocks {
			for ii := range c.f.Blocks[bi].Insts {
				in := &c.f.Blocks[bi].Insts[ii]
				if in.Op == OpCellSet || in.Op == OpSetField {
					c.viewDefs[in.Args[0]] = append(c.viewDefs[in.Args[0]], in)
				}
			}
		}
		c.viewSets = map[LocalID]viewSourceSet{}
	}
	s := viewSourceSet{complete: true}
	add := func(l LocalID) {
		if !slices.Contains(s.locals, l) {
			s.locals = append(s.locals, l)
		}
	}
	type visit struct {
		local LocalID
		path  string
	}
	seen, work := map[visit]bool{}, []viewHop{{local: a}}
	for len(work) > 0 {
		h := work[len(work)-1]
		work = work[:len(work)-1]
		key := visit{h.local, fmt.Sprint(h.path)}
		if seen[key] {
			continue
		}
		seen[key] = true
		if len(seen) > 4*len(c.f.Locals) {
			s.complete = false
			break
		}
		defs := c.viewDefs[h.local]
		if len(defs) == 0 {
			if len(h.path) == 0 && slices.Contains(c.f.Params, h.local) && c.viewType(c.f.Locals[h.local].Type) {
				add(h.local)
			} else {
				s.complete = false
			}
		}
		for _, d := range defs {
			if d.Op == OpVariant && len(d.Args) == 0 {
				continue
			}
			if len(d.Args) == 0 {
				s.complete = false
				continue
			}
			next := viewHop{local: d.Args[0], path: h.path}
			switch d.Op {
			case OpLendBox:
				if len(h.path) == 0 {
					add(d.Args[0])
				} else {
					s.complete = false
				}
				continue
			case OpField, OpPayload:
				next.path = append([]int{d.Index}, h.path...)
			case OpRecord, OpVariant:
				if len(h.path) == 0 || h.path[0] < 0 {
					s.complete = false
					continue
				}
				if h.path[0] >= len(d.Args) {
					s.complete = s.complete && d.Op == OpVariant
					continue
				}
				next.local, next.path = d.Args[h.path[0]], h.path[1:]
			case OpSetField:
				if len(h.path) == 0 || h.path[0] != d.Index {
					continue
				}
				next.local, next.path = d.Args[len(d.Args)-1], h.path[1:]
			case OpCellSet:
				next.local = d.Args[len(d.Args)-1]
			case OpCopy, OpMove, OpAlias, OpBorrow, OpShare, OpNewCell, OpCellGet, OpCellTake:
			default:
				s.complete = false
				continue
			}
			work = append(work, next)
		}
	}
	s.complete = s.complete && len(s.locals) > 0
	if !s.complete {
		add(a)
	}
	c.viewSets[a] = s
	return s
}

func (c *borrowCheck) keptMutTargets(a LocalID) []storeFlow {
	s := c.viewSources(a)
	var out []storeFlow
	for _, l := range s.locals {
		if c.keptPlace(l) || !s.complete && l == a {
			out = append(out, storeFlow{target: l, ref: true, kept: true, unresolved: !s.complete && l == a})
		}
	}
	return out
}
