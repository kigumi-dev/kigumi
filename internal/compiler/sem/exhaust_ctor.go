package sem

import (
	"kigumi/internal/compiler/syntax"
)

// ctorEntry is a column type's constructors, memoized for the check: every arm
// of a match and every variant of an exhaustive walk asks for the same ADT, and
// rebuilding a variant list per arm is quadratic in the arm count.
type ctorEntry struct {
	set      []ctorDesc
	complete bool
	byKey    map[string]int
}

// ctorSet lists the constructors of a column type; complete is false for
// unbounded domains (numbers, strings, existentials, opaque types).
func (c *checker) ctorSet(t TypeID) ([]ctorDesc, bool) {
	e := c.ctorEntry(t)
	return e.set, e.complete
}

func (c *checker) ctorEntry(t TypeID) *ctorEntry {
	t = c.vars.resolve(t)
	if e, ok := c.r.ctorMemo[t]; ok {
		return e
	}
	set, complete := c.buildCtorSet(t)
	e := &ctorEntry{set: set, complete: complete, byKey: make(map[string]int, len(set))}
	for i, d := range set {
		e.byKey[d.key] = i
	}
	if c.r.ctorMemo == nil {
		c.r.ctorMemo = map[TypeID]*ctorEntry{}
	}
	c.r.ctorMemo[t] = e
	return e
}

func (c *checker) buildCtorSet(t TypeID) ([]ctorDesc, bool) {
	tt := c.r.Types
	if t == TyBool {
		return []ctorDesc{{key: "true", name: "true"}, {key: "false", name: "false"}}, true
	}
	n := tt.Node(t)
	if n.Kind != KNamed {
		return nil, false
	}
	info := c.r.typeDecl(n.Ent)
	subst := map[EntityID]TypeID{}
	for i, p := range info.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	switch info.Form {
	case FormAdt:
		var out []ctorDesc
		for _, v := range info.Variants {
			var args []TypeID
			for _, p := range c.r.variant(v).Payload {
				args = append(args, tt.Subst(p, subst))
			}
			out = append(out, ctorDesc{key: variantKey(v), name: c.r.Entities[v].Name, args: args})
		}
		return out, true
	case FormRecord, FormResource:
		var args []TypeID
		for _, f := range info.Fields {
			args = append(args, tt.Subst(c.r.Entities[f].Type, subst))
		}
		return []ctorDesc{{key: recordKey(n.Ent), name: c.r.Entities[n.Ent].Name, args: args}}, true
	}
	return nil, false
}

func variantKey(v EntityID) string { return "v" + itoa(int(v)) }
func recordKey(e EntityID) string  { return "r" + itoa(int(e)) }

// argTypes gives the column types introduced by specializing on h.
func (c *checker) argTypes(t TypeID, h pat) []TypeID {
	e := c.ctorEntry(t)
	if i, ok := e.byKey[h.key]; ok {
		return e.set[i].args
	}
	return wildTypes(len(h.args))
}

func wildTypes(n int) []TypeID {
	out := make([]TypeID, n)
	for i := range out {
		out[i] = TyPoison
	}
	return out
}

// abstract turns a checked pattern node into a matrix pattern.
func (c *checker) abstract(p syntax.NodeID) pat {
	v := c.t.View(p)
	if c.info.Types[p] == TyPoison {
		return pat{kind: pErr}
	}
	switch v.Kind() {
	case syntax.PatLit:
		return c.abstractLit(v.AsPatLit().Lit().ID())
	case syntax.PatCtor:
		ent := c.info.Uses[p]
		if ent == 0 || c.r.Entities[ent].Kind != EntVariant {
			if ent != 0 {
				return pat{kind: pLit, key: "t" + itoa(int(ent)), name: c.r.Entities[ent].Name}
			}
			return pat{kind: pWild}
		}
		vi := c.r.variant(ent)
		args := wilds(len(vi.Payload))
		for i, s := range v.AsPatCtor().Args().EnumerateIDs() {
			if i < len(args) {
				args[i] = c.abstract(s)
			}
		}
		return pat{kind: pCtor, key: variantKey(ent), name: c.r.Entities[ent].Name, args: args}
	case syntax.PatRecord:
		return c.abstractRecord(p)
	case syntax.PatTuple:
		return c.abstractTuple(p)
	case syntax.PatRange:
		// Each range gets its own key: ranges never make an integer match
		// exhaustive (defaultMatrix already excludes non-pWild rows), and
		// overlap between ranges, or a range and a literal, is not an error
		// so a range's key never collides with another row's.
		return pat{kind: pRange, key: "g" + itoa(int(p))}
	}
	return pat{kind: pWild}
}

// abstractTuple orders the field patterns by position: a tuple pattern
// always names every position, so unlike abstractRecord there is nothing
// left as an implicit wildcard.
func (c *checker) abstractTuple(p syntax.NodeID) pat {
	ent := c.info.Uses[p]
	if ent == 0 {
		return pat{kind: pWild}
	}
	args := make([]pat, 0, c.t.List(p).Len())
	for s := range c.t.List(p).IDs() {
		args = append(args, c.abstract(s))
	}
	return pat{kind: pCtor, key: recordKey(ent), name: c.r.Entities[ent].Name, args: args}
}

func (c *checker) abstractLit(lit syntax.NodeID) pat {
	lit = c.literalNode(lit)
	text := c.t.View(lit).Tok().Text()
	if c.t.Kind(lit) == syntax.BoolLit {
		return pat{kind: pCtor, key: text, name: text}
	}
	if v, ok := c.info.Literals[lit]; ok && v.Kind == constInt {
		text = v.Int.String()
	}
	return pat{kind: pLit, key: "l" + text, name: text}
}

// abstractRecord orders the field patterns by declaration (records) or by
// payload position (named variants); unmentioned fields are wildcards.
func (c *checker) abstractRecord(p syntax.NodeID) pat {
	ent := c.info.Uses[p]
	if ent == 0 {
		return pat{kind: pWild}
	}
	subs := map[string]pat{}
	for fv := range c.t.View(p).AsPatRecord().Fields().Views() {
		f := fv.AsPatField()
		if f.Sub().Valid() {
			subs[f.Name().Text()] = c.abstract(f.Sub().ID())
		}
	}
	return c.recordPat(ent, subs)
}

// recordPat assembles a record or named-variant constructor pattern from
// sub-patterns keyed by field name.
func (c *checker) recordPat(ent EntityID, subs map[string]pat) pat {
	e := &c.r.Entities[ent]
	if e.Kind == EntVariant {
		vi := c.r.variant(ent)
		args := wilds(len(vi.Payload))
		for i, name := range vi.Names {
			if s, ok := subs[name]; ok {
				args[i] = s
			}
		}
		return pat{kind: pCtor, key: variantKey(ent), name: e.Name, args: args}
	}
	fields := c.r.typeDecl(ent).Fields
	args := wilds(len(fields))
	for i, f := range fields {
		if s, ok := subs[c.r.Entities[f].Name]; ok {
			args[i] = s
		}
	}
	return pat{kind: pCtor, key: recordKey(ent), name: e.Name, args: args}
}
