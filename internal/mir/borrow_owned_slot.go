package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// borrowOwnedTupleElem is borrow-owned-field's message for a tuple element:
// a built-in TupleN has no `['a]` of its own for an element to route through.
const borrowOwnedTupleElem = "a borrow cannot go in `{name}`, an element of `{type}` declared `{type2}`: an element declared that way is released with the tuple, and this borrow has no count to give; a tuple cannot take a `['a]` of its own, so put it in a record that can, or store a value the tuple can own"

// Only a field declared `&'a T`, or one routing a borrow through its own `'a`,
// is skipped by the destructor and by rt_copy; a type parameter is not.
func (c *borrowCheck) checkHolderFields(in Inst) {
	if in.Ent == 0 || c.p.R.Entity(in.Ent).Kind != sem.EntType {
		return
	}
	fields := c.p.R.TypeDecl(in.Ent).Fields
	if len(fields) != len(in.Args) {
		return
	}
	tuple := c.p.R.Types.IsTupleEnt(in.Ent)
	for i, a := range in.Args {
		t := c.p.R.Entity(fields[i]).Type
		if !c.unownedRef(a) || c.declaredAsBorrow(t) {
			continue
		}
		if tuple {
			c.reportTemplate(in.Node, a, "E1531", borrowOwnedTupleElem, c.p.R.Entity(fields[i]).Name, c.p.R.Entity(in.Ent).Name, c.p.R.TypeString(t))
			continue
		}
		c.reportCode(in.Node, a, "borrow-owned-field", c.p.R.Entity(fields[i]).Name, c.p.R.Entity(in.Ent).Name, c.p.R.TypeString(t))
	}
}

// A parameter declared as a type parameter is dropped by the callee, however
// that parameter is later instantiated.
func (c *borrowCheck) checkOwningArgs(in Inst) {
	for i, a := range in.Args {
		if !c.unownedRef(a) {
			continue
		}
		prm, ok := c.p.declaredParamEnt(in, i)
		if !ok {
			continue
		}
		t := c.p.R.Entity(prm).Type
		if c.declaredAsBorrow(t) {
			continue
		}
		c.reportCode(in.Node, a, "borrow-owned-param", c.p.R.Entity(prm).Name, c.p.R.Entity(in.Ent).Name, c.p.R.TypeString(t))
	}
}

func (c *borrowCheck) declaredAsBorrow(t sem.TypeID) bool {
	return c.p.R.Types.Kind(t) == sem.KRef || c.declaredLifetime(t) != 0
}

// A field read retains, so only a borrow and a lent parameter are unowned.
func (c *borrowCheck) unownedRef(l LocalID) bool {
	if c.unowned == nil {
		c.unowned = c.p.unownedRefs(c.f)
	}
	return c.unowned[l]
}

// The help is folded into the message, the way the lifetime rules render theirs.
func (c *borrowCheck) reportCode(n syntax.NodeID, local LocalID, name string, args ...any) {
	code, ok := sem.CodeByName(name)
	if !ok {
		return
	}
	msg := sem.FillTemplate(code.Template, args...)
	if code.Help != "" {
		msg += "; " + code.Help
	}
	c.report(n, local, code.Num, msg)
}

// reportCatalog is reportCode with the help on a line of its own, split by sem.FillCode as sem reports the code.
func (c *borrowCheck) reportCatalog(n syntax.NodeID, local LocalID, name string, args ...any) {
	code, ok := sem.CodeByName(name)
	if !ok {
		return
	}
	msg, help := sem.FillCode(code, args...)
	c.reportHelped(n, local, code.Num, msg, 0, "", help)
}

// reportTemplate is reportCode for a message that varies by shape (E1531's
// tuple element vs. record field): template already carries its help text.
func (c *borrowCheck) reportTemplate(n syntax.NodeID, local LocalID, num, template string, args ...any) {
	c.report(n, local, num, sem.FillTemplate(template, args...))
}
