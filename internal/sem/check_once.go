package sem

import (
	"maps"

	"kigumi/internal/syntax"
)

// onceFlow tracks FnOnce-bound callees: called maps those some path has already called to that call, loops the open loops.
type onceFlow struct {
	called map[EntityID]syntax.NodeID
	loops  []*onceLoop
}

type onceState = map[EntityID]syntax.NodeID

type onceLoop struct {
	scope        ScopeID
	entry, start onceState
	back, out    []onceState
}

type onceJoin struct {
	entry onceState
	exits []onceState
}

func (c *checker) onceBegin() *onceJoin {
	return &onceJoin{entry: maps.Clone(c.once.called)}
}

func (c *checker) onceArm(j *onceJoin, t TypeID) {
	if c.vars.resolve(t) != TyNever {
		j.exits = append(j.exits, c.once.called)
	}
	c.once.called = maps.Clone(j.entry)
}

func (c *checker) onceEnd(j *onceJoin, skipped bool) {
	if skipped {
		j.exits = append(j.exits, j.entry)
	}
	merged := maps.Clone(j.entry)
	c.once.called = unionOnce(merged, j.exits...)
}

func unionOnce(dst onceState, srcs ...onceState) onceState {
	for _, src := range srcs {
		if dst == nil {
			dst = onceState{}
		}
		for k, v := range src {
			if _, ok := dst[k]; !ok {
				dst[k] = v
			}
		}
	}
	return dst
}

func (c *checker) onceLoopBegin() *onceLoop {
	lp := &onceLoop{scope: c.scope, entry: maps.Clone(c.once.called)}
	c.once.loops = append(c.once.loops, lp)
	return lp
}

func (c *checker) onceLoopBody(lp *onceLoop) {
	lp.start = maps.Clone(c.once.called)
}

// A callee an outer scope owns that a back edge still holds as called would be called again next iteration.
func (c *checker) onceLoopEnd(lp *onceLoop, body TypeID, bare bool) {
	c.once.loops = c.once.loops[:len(c.once.loops)-1]
	back := lp.back
	if c.vars.resolve(body) != TyNever {
		back = append(back, maps.Clone(c.once.called))
	}
	reported := map[EntityID]bool{}
	for _, st := range back {
		for ent, site := range st {
			if _, in := lp.entry[ent]; in || reported[ent] || c.scopeWithin(c.r.local(ent).Scope, lp.scope) {
				continue
			}
			reported[ent] = true
			c.errAt(site, cOnceCalledInLoop, c.r.Entities[ent].Name)
		}
	}
	after := lp.out
	if !bare {
		after = append(append(after, lp.start), back...)
	}
	c.once.called = unionOnce(nil, after...)
}

func (c *checker) onceLeave(isBreak bool) {
	if len(c.once.loops) == 0 {
		return
	}
	lp := c.once.loops[len(c.once.loops)-1]
	if isBreak {
		lp.out = append(lp.out, maps.Clone(c.once.called))
	} else {
		lp.back = append(lp.back, maps.Clone(c.once.called))
	}
}

func (c *checker) onlyFnOnce(t TypeID) bool {
	if c.r.Types.Kind(t) != KParam {
		return false
	}
	once := false
	for _, con := range c.r.typeParam(c.r.Types.Node(t).Ent).Constraints {
		switch con.Kind {
		case CFn, CFnMut:
			return false
		case CFnOnce:
			once = true
		}
	}
	return once
}

// A use is a call or passing the callee on; a use inside a closure counts as repeatable because the closure may run many times.
func (c *checker) noteOnceUse(n, callee syntax.NodeID, ft TypeID) {
	if !c.onlyFnOnce(c.vars.resolve(ft)) {
		return
	}
	for c.t.Kind(callee) == syntax.Paren {
		callee = c.t.View(callee).AsParen().Inner().ID()
	}
	ent := c.info.Uses[callee]
	if ent == 0 || c.t.Kind(callee) != syntax.Ident {
		return
	}
	if k := c.r.Entities[ent].Kind; k != EntLocal && k != EntParam {
		return
	}
	name := c.r.Entities[ent].Name
	_, again := c.once.called[ent]
	switch {
	case len(c.lambdas) > 0 && !c.scopeWithin(c.r.local(ent).Scope, c.lambdas[len(c.lambdas)-1].scope):
		c.errAt(n, cOnceCalledInLoop, name)
	case again:
		c.errAt(n, cOnceCalledAgain, name)
	}
	if c.once.called == nil {
		c.once.called = onceState{}
	}
	c.once.called[ent] = n
}

func (c *checker) onceReassign(lhs syntax.NodeID) {
	for c.t.Kind(lhs) == syntax.Paren {
		lhs = c.t.View(lhs).AsParen().Inner().ID()
	}
	if c.t.Kind(lhs) == syntax.Ident {
		delete(c.once.called, c.info.Uses[lhs])
	}
}
