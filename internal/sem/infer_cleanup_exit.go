package sem

import "kigumi/internal/syntax"

// exitMoveState lets a success exit move out a binding only a defer(fail) reserves.
type exitMoveState struct {
	tail      syntax.NodeID
	returning []exitSite
	moves     []exitMove
}

type exitSite struct {
	value   syntax.NodeID
	lambdas int
}

type exitMove struct {
	use  syntax.NodeID
	ent  EntityID
	res  reservation
	exit syntax.NodeID
}

func (c *checker) bodyTail(body syntax.NodeID) syntax.NodeID {
	if c.t.Kind(body) != syntax.Block {
		return 0
	}
	return c.t.View(body).AsBlock().Tail().ID()
}

// exitMoveCandidate records a move of a defer(fail)-reserved binding inside a success exit's value; checkExitMoves decides it.
func (c *checker) exitMoveCandidate(a syntax.NodeID, ent EntityID, r reservation) bool {
	if c.cleanup > 0 || !r.fail || r.lambdas != len(c.lambdas) {
		return false
	}
	exit := syntax.NodeID(0)
	if s := c.exitCheck.returning; len(s) > 0 && s[len(s)-1].lambdas == len(c.lambdas) && c.within(a, s[len(s)-1].value) {
		exit = s[len(s)-1].value
	} else if len(c.lambdas) == 0 && c.exitCheck.tail != 0 && c.within(a, c.exitCheck.tail) {
		exit = c.exitCheck.tail
	}
	if exit == 0 {
		return false
	}
	c.exitCheck.moves = append(c.exitCheck.moves, exitMove{use: a, ent: ent, res: r, exit: exit})
	return true
}

func (c *checker) within(inner, outer syntax.NodeID) bool {
	i, o := c.t.Span(inner), c.t.Span(outer)
	return i.Start >= o.Start && i.End <= o.End
}

func (c *checker) checkExitMoves() {
	for _, m := range c.exitCheck.moves {
		if !c.staticOk(m.exit) || !c.noFailureAfter(m.exit, m.use) {
			c.reservedErrorAt(m.use, cCleanupReservedMove, m.ent, m.res)
		}
	}
	c.exitCheck.moves = nil
}

// staticOk reports whether every value n yields is a success: a non-Result value the exit wraps in Ok, or an `Ok(...)`.
func (c *checker) staticOk(n syntax.NodeID) bool {
	if _, _, isResult := c.r.Types.IsResult(c.vars.resolve(c.info.Types[n])); !isResult {
		return true
	}
	switch c.t.Kind(n) {
	case syntax.Paren:
		return c.staticOk(c.t.View(n).AsParen().Inner().ID())
	case syntax.Block:
		tail := c.bodyTail(n)
		return tail != 0 && c.staticOk(tail)
	case syntax.IfExpr:
		s := ifExpr(c.t, n)
		return s.Else != 0 && c.staticOk(s.Then) && c.staticOk(s.Else)
	case syntax.IfLetExpr:
		s := ifLet(c.t, n)
		return s.Else != 0 && c.staticOk(s.Then) && c.staticOk(s.Else)
	case syntax.MatchExpr:
		for arm := range c.t.View(n).AsMatch().Arms().IDs() {
			if !c.staticOk(matchArm(c.t, arm).Body) {
				return false
			}
		}
		return true
	case syntax.CallExpr:
		call, ok := c.info.Calls[n]
		if !ok || call.Kind != CallVariant {
			return false
		}
		e := &c.r.Entities[call.Callee]
		return e.Kind == EntVariant && e.Parent == c.r.Types.resultEnt && e.Name == "Ok"
	}
	return false
}

// noFailureAfter rejects any `?`, `fail` or nested `return` in n that can run after use, and a use inside a loop or lambda.
func (c *checker) noFailureAfter(n, use syntax.NodeID) bool {
	u := c.t.Span(use)
	sp := c.t.Span(n)
	switch c.t.Kind(n) {
	case syntax.Lambda:
		return !c.within(use, n)
	case syntax.ForExpr:
		if c.within(use, n) {
			return false
		}
	case syntax.TryExpr:
		if sp.End > u.End {
			return false
		}
	case syntax.FailExpr, syntax.ReturnExpr:
		if sp.End > u.Start {
			return false
		}
	}
	for ch := range c.t.View(n).Children() {
		if !c.noFailureAfter(ch.ID(), use) {
			return false
		}
	}
	return true
}
