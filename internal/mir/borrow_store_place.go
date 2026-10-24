package mir

import "kigumi/internal/sem"

func (c *borrowCheck) untrackedPlace(fl storeFlow) string {
	l := fl.target
	for c.f.Locals[l].Ent == 0 {
		in := c.definition(l)
		if in == nil || len(in.Args) == 0 || in.Op != OpBorrow && in.Op != OpNewCell && in.Op != OpShare && in.Op != OpCopy && in.Op != OpMove && in.Op != OpAlias {
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
