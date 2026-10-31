package sem

import "kigumi/internal/compiler/syntax"

// Bodiless std methods that hand content of the receiver to the caller.
var stdDrainsReceiver = map[string]bool{"array.Array.takeAt": true, "prelude.Option.take": true}

// A cleanup that empties a place consumes the taken content, so a later assignment must not drop it unclosed.
func (c *checker) reserveMutDrain(base syntax.NodeID, m EntityID) {
	if c.cleanup > 0 && c.drainsReceiver(m, map[EntityID]bool{}) {
		c.reserveUse(base)
	}
}

func (c *checker) drainsReceiver(m EntityID, seen map[EntityID]bool) bool {
	if seen[m] {
		return false
	}
	seen[m] = true
	info := c.r.Fn(m)
	if info.Body == 0 {
		return c.r.Entities[m].Flags&EfStd != 0 && stdDrainsReceiver[c.r.stdKey(m)]
	}
	return info.Owner != 0 && c.bodyDrains(drainScan{c.r.tree(c.r.Entities[m].File), "self", info.Owner, m}, info.Body, seen)
}

func (c *checker) paramDrains(param EntityID, seen map[EntityID]bool) bool {
	pe := &c.r.Entities[param]
	if pe.Kind != EntParam || seen[param] || c.r.Entities[pe.Parent].Kind != EntFn {
		return false
	}
	seen[param] = true
	info := c.r.Fn(pe.Parent)
	if info.Body == 0 || c.r.Types.Kind(pe.Type) != KRef {
		return false
	}
	s := selfScan{c: c}
	d := drainScan{c.r.tree(pe.File), pe.Name, s.typeEnt(c.r.Types.Node(pe.Type).Elem), pe.Parent}
	return c.bodyDrains(d, info.Body, seen)
}

type drainScan struct {
	t     *syntax.Tree
	root  string
	owner EntityID
	fn    EntityID
}

func (c *checker) bodyDrains(d drainScan, n syntax.NodeID, seen map[EntityID]bool) bool {
	t := d.t
	v := t.View(n)
	if v.Kind() == syntax.CallExpr {
		switch callee := t.Unparen(v.AsCall().Callee().ID()); t.Kind(callee) {
		case syntax.MemberExpr:
			mem := t.View(callee).AsMember()
			if rootedAt(t, mem.Base().ID(), d.root) && c.selfMethodDrains(mem.Tok().Text(), d.owner, seen) {
				return true
			}
		case syntax.Ident:
			if c.passesToDrain(d, v.AsCall(), t.View(callee).Tok().Text(), seen) {
				return true
			}
		}
	}
	for ch := range v.Children() {
		if c.bodyDrains(d, ch.ID(), seen) {
			return true
		}
	}
	return false
}

func (c *checker) passesToDrain(d drainScan, call syntax.CallView, name string, seen map[EntityID]bool) bool {
	s := selfScan{c: c, fn: d.fn}
	fn := s.pkgFn(d.t, name)
	if fn == 0 {
		return false
	}
	params := c.r.Fn(fn).Params
	i := 0
	for a := range call.Args().IDs() {
		if arg := d.t.Unparen(a); i < len(params) && d.passesRoot(arg) && c.paramDrains(params[i], seen) {
			return true
		}
		i++
	}
	return false
}

func (c *checker) selfMethodDrains(name string, owner EntityID, seen map[EntityID]bool) bool {
	if name == "takeAt" || name == "take" {
		return true
	}
	if owner == 0 {
		return false
	}
	set := c.r.typeDecl(owner).Members[name]
	if set == 0 {
		return false
	}
	for _, mm := range c.r.Overloads[set].Members {
		if c.r.Fn(mm).Recv == RecvMut && c.drainsReceiver(mm, seen) {
			return true
		}
	}
	return false
}

func rootedAt(t *syntax.Tree, n syntax.NodeID, root string) bool {
	for {
		switch t.Kind(n) {
		case syntax.Ident:
			return t.View(n).Tok().Text() == root
		case syntax.Paren:
			n = t.View(n).AsParen().Inner().ID()
		case syntax.MemberExpr:
			n = t.View(n).AsMember().Base().ID()
		case syntax.BracketExpr:
			n = t.View(n).AsBracket().Base().ID()
		default:
			return false
		}
	}
}

func (d drainScan) passesRoot(arg syntax.NodeID) bool {
	if d.t.Kind(arg) == syntax.Ident {
		return d.t.View(arg).Tok().Text() == d.root
	}
	if d.t.Kind(arg) != syntax.BorrowExpr || d.t.View(arg).AsBorrow().Flags()&syntax.FlagMut == 0 {
		return false
	}
	return rootedAt(d.t, d.t.View(arg).AsBorrow().Operand().ID(), d.root)
}
