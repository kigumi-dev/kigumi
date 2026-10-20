package sem

import "kigumi/internal/syntax"

func (c *checker) synthMatch(n syntax.NodeID, want TypeID) TypeID {
	m := c.t.View(n).AsMatch()
	subject := m.Scrutinee().ID()
	valuePos := c.stmtPos != n
	scrutinee := c.synth(subject)
	if c.r.Types.Kind(c.vars.resolve(scrutinee)) == KUntyped {
		scrutinee = c.adopt(c.literalNode(subject), defaultOf(c.vars.resolve(scrutinee)))
	}
	arms := m.Arms().AppendIDs(nil)
	var bodies []syntax.NodeID
	var types []TypeID
	entry := c.snapshotFacts()
	c.info.Narrow[n] = entry
	place := c.factPlace(subject)
	var excluded []NarrowFact
	var joined []NarrowFact
	live := 0
	scrutineeRp := c.restrictedAliasOf(subject)
	once := c.onceBegin()
	for _, arm := range arms {
		s := matchArm(c.t, arm)
		saved := c.scope
		c.facts = append(c.snapshotFacts()[:0], entry...)
		c.addFacts(excluded)
		c.pushScope(ScopeArm, arm)
		c.checkPatternOn(s.Pattern, subject, scrutinee, false, c.scope)
		c.taintPatternBindings(s.Pattern, scrutineeRp)
		c.notePatternAliases(s.Pattern, subject, true)
		pos, neg := c.patternFacts(s.Pattern, place)
		c.addFacts(pos)
		if s.Guard != 0 {
			gp, _ := c.condFacts(s.Guard)
			c.addFacts(gp)
		} else {
			excluded = append(excluded, neg...)
		}
		t := c.checkBranch(n, s.Body, want, valuePos)
		c.onceArm(once, t)
		c.popScope(saved)
		if c.vars.resolve(t) != TyNever {
			if live == 0 {
				joined = c.snapshotFacts()
			} else {
				joined = intersectFacts(joined, c.facts)
			}
			live++
		}
		bodies = append(bodies, s.Body)
		types = append(types, t)
	}
	c.onceEnd(once, false)
	c.facts = entry
	c.checkMatch(n, subject, c.scrutineeOf(scrutinee), arms)
	if live > 0 && c.info.Matches[n].Exhaustive {
		c.facts = joined
	}
	return c.branchResult(want, valuePos, bodies, types)
}
