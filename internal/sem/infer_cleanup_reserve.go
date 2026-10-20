package sem

import (
	"slices"
	"strings"

	"kigumi/internal/syntax"
)

// reservation holds the place a cleanup consumes: fields is the path below the binding, empty for the whole binding.
type reservation struct {
	cleanup syntax.NodeID
	scope   ScopeID
	fail    bool
	lambdas int
	fields  []EntityID
	name    string
}

type reservedPlace struct {
	ent    EntityID
	fields []EntityID
	node   syntax.NodeID
}

type cleanupLog struct {
	consumed []reservedPlace
	assigned []assignSite
	earlier  []assignSite
}

type assignSite struct {
	place reservedPlace
	node  syntax.NodeID
	scope ScopeID
}

func (c *checker) ownsType(t TypeID) bool {
	t = c.vars.resolve(t)
	return t != TyPoison && c.r.Types.Kind(t) != KRef && !c.r.isCopy(t)
}

func (c *checker) ownsValue(ent EntityID) bool {
	e := &c.r.Entities[ent]
	return (e.Kind == EntLocal || e.Kind == EntParam) && c.ownsType(e.Type)
}

func (c *checker) declaredIn(ent EntityID, n syntax.NodeID) bool {
	e := &c.r.Entities[ent]
	if e.File != c.f {
		return false
	}
	inner, outer := c.t.Span(e.Node), c.t.Span(n)
	return inner.Start >= outer.Start && inner.End <= outer.End
}

func (c *checker) placeUnder(n syntax.NodeID) (reservedPlace, bool) {
	p, ok := c.buildPlace(c.stripParen(n))
	if !ok || p.Root == 0 || p.Index || p.Deref {
		return reservedPlace{}, false
	}
	return reservedPlace{ent: p.Root, fields: p.Fields}, true
}

func (c *checker) placeLabel(p reservedPlace) string {
	var sb strings.Builder
	sb.WriteString(c.r.Entities[p.ent].Name)
	for _, f := range p.fields {
		sb.WriteString("." + c.r.Entities[f].Name)
	}
	return sb.String()
}

func fieldPathsOverlap(a, b []EntityID) bool {
	return isFieldPrefix(a, b) || isFieldPrefix(b, a)
}

func isFieldPrefix(prefix, path []EntityID) bool {
	return len(prefix) <= len(path) && slices.Equal(prefix, path[:len(prefix)])
}

func (c *checker) reservedOver(ent EntityID, fields []EntityID) (int, bool) {
	return c.reservedWhere(ent, fieldPathsOverlap, fields)
}

func (c *checker) reservedWithin(ent EntityID, fields []EntityID) (int, bool) {
	return c.reservedWhere(ent, isFieldPrefix, fields)
}

func (c *checker) reservedWhere(ent EntityID, match func(a, b []EntityID) bool, fields []EntityID) (int, bool) {
	for i, r := range c.reserved[ent] {
		if match(fields, r.fields) {
			return i, true
		}
	}
	return 0, false
}

func (c *checker) reservedError(n syntax.NodeID, code Code, r reservation) {
	d, ok := c.r.diagAt(c.f, n, code, r.name)
	if ok {
		c.r.emitDiag(c.f, d.WithNote(c.loc(r.cleanup), c.reserverNote(r)))
	}
}

func (c *checker) holdReservations(n syntax.NodeID, places []reservedPlace) {
	for _, p := range places {
		if _, held := c.reservedOver(p.ent, p.fields); held {
			continue
		}
		if c.reserved == nil {
			c.reserved = map[EntityID][]reservation{}
		}
		c.reserved[p.ent] = append(c.reserved[p.ent], reservation{
			cleanup: n, scope: c.scope, fail: c.t.Kind(n) == syntax.DeferFailStmt,
			lambdas: len(c.lambdas), fields: p.fields, name: c.placeLabel(p),
		})
	}
}

func (c *checker) releaseReservations(scope ScopeID) {
	c.cleanupLog.earlier = slices.DeleteFunc(c.cleanupLog.earlier, func(a assignSite) bool { return a.scope == scope })
	for ent, rs := range c.reserved {
		kept := rs[:0]
		for _, r := range rs {
			if r.scope != scope {
				kept = append(kept, r)
			}
		}
		if len(kept) == 0 {
			delete(c.reserved, ent)
		} else {
			c.reserved[ent] = kept
		}
	}
}
