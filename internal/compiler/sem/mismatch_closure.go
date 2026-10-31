package sem

import (
	"strings"

	"kigumi/internal/compiler/syntax"
)

func (c *checker) closureMismatch(n syntax.NodeID, code Code, a, b TypeID) bool {
	tt := c.r.Types
	a, b = c.vars.resolve(a), c.vars.resolve(b)
	if c.adoptMethodClosure(n, a, b) {
		return true
	}
	na, nb := tt.Node(a), tt.Node(b)
	if na.Kind != KClosure || nb.Kind != KClosure || na.Ent == nb.Ent || c.r.TypeString(a) != c.r.TypeString(b) {
		return false
	}
	d, ok := c.r.diagAt(c.f, n, code, c.r.TypeString(a), c.r.TypeString(b))
	if !ok {
		return true
	}
	for i, id := range []EntityID{na.Ent, nb.Ent} {
		e := &c.r.Entities[id]
		if e.File != 0 && e.Node != 0 {
			d = d.WithNote(c.r.loc(e.File, c.r.Files[e.File].span(e.Node)), c.closureOrigin(id, []EntityID{nb.Ent, na.Ent}[i]))
		}
	}
	c.r.emitDiag(c.f, d)
	if ia, ib := c.r.closure(na.Ent), c.r.closure(nb.Ent); ia.IsMethodValue() && ib.IsMethodValue() {
		c.vars.unify(ia.Sig, ib.Sig)
	}
	return true
}

func (c *checker) closureOrigin(id, other EntityID) string {
	info := c.r.closure(id)
	if info.IsMethodValue() {
		m := c.r.Entities[info.Method].Name
		recv := c.r.TypeString(c.r.Entities[info.Captures[0].Local].Type)
		return "one closure type is the method value `" + m + "` holding a `" + recv + "` receiver" + c.methodClosureDiff(info, c.r.closure(other))
	}
	var names []string
	for _, cp := range info.Captures {
		names = append(names, "`"+c.r.Entities[cp.Local].Name+"`")
	}
	if len(names) == 0 {
		return "one closure type is this lambda, which captures nothing; each lambda has its own type"
	}
	return "one closure type is this lambda, capturing " + strings.Join(names, ", ") + "; each lambda has its own type"
}

func (c *checker) methodClosureDiff(a, b *ClosureInfo) string {
	if !b.IsMethodValue() || a.Method != b.Method || c.r.Entities[a.Captures[0].Local].Type != c.r.Entities[b.Captures[0].Local].Type {
		return ""
	}
	switch {
	case a.Creator != b.Creator && c.r.Entities[a.Creator].Kind == EntClosure:
		return ", created inside a lambda, which gives it its own type"
	case a.Creator != b.Creator:
		return ", created in `" + c.r.Entities[a.Creator].Name + "` outside the lambda"
	case a.Scoped != b.Scoped && a.Scoped:
		return ", created inside an allocator block, which gives it its own type"
	case a.Scoped != b.Scoped:
		return ", created outside the allocator block"
	}
	return ", created while its type arguments were still open; fix them with a call or an `F: Fn[...]` parameter before the two meet"
}
