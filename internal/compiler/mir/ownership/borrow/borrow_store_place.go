package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) untrackedPlace(fl storeFlow) string {
	l := fl.target
	for c.f.Locals[l].Ent == 0 {
		in := c.definition(l)
		if in == nil || len(in.Args) == 0 || in.Op != midir.OpBorrow && in.Op != midir.OpNewCell && in.Op != midir.OpShare && in.Op != midir.OpCopy && in.Op != midir.OpMove && in.Op != midir.OpAlias {
			break
		}
		l = in.Args[0]
	}
	t := c.f.Locals[l].Type
	for c.p.R.Types.Kind(t) == sem.KRef {
		t = c.p.R.Types.Node(t).Elem
	}
	if c.f.Locals[l].Ent == 0 || c.f.Locals[l].Name == "" {
		return c.typeName(t)
	}
	return c.typeName(t) + " behind `" + c.f.Locals[l].Name + "`"
}

func (c *borrowCheck) typeName(t sem.TypeID) string {
	if t == 0 {
		return "this place"
	}
	return "`" + c.p.R.TypeString(t) + "`"
}
