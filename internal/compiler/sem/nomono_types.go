package sem

import (
	"strings"

	"kigumi/internal/compiler/syntax"
)

type typeVertex struct {
	decl  EntityID
	param int
}

type typeEdge struct {
	from, to typeVertex
	grow     bool
	owner    EntityID
	site     EntityID
	arg      TypeID
}

// checkTypeNomono rejects a type whose declaration needs itself at ever larger arguments, such as
// `type G[T] = { g Array[G[Option[T]]] }`, before any walk of its fields can run forever.
func (r *Result) checkTypeNomono() {
	g := &instGraph{r: r, index: map[instVertex]int{}, adj: map[int][]int{}}
	var edges []typeEdge
	for id := 1; id < len(r.Entities); id++ {
		if r.Entities[id].Kind == EntType {
			edges = r.typeEdges(EntityID(id), edges)
		}
	}
	for _, e := range edges {
		from := g.vertex(instVertex{e.from.decl, e.from.param})
		to := g.vertex(instVertex{e.to.decl, e.to.param})
		g.adj[from] = append(g.adj[from], to)
	}
	comp := g.scc()
	bad := map[EntityID]bool{}
	for _, e := range edges {
		if !e.grow || comp[g.index[instVertex{e.from.decl, e.from.param}]] != comp[g.index[instVertex{e.to.decl, e.to.param}]] || bad[e.owner] {
			continue
		}
		bad[e.owner] = true
		decl := r.typeDecl(e.from.decl)
		from := r.Entities[e.from.decl].Name + "[" + r.Entities[decl.Params[e.from.param]].Name + "]"
		to := r.Entities[e.to.decl].Name + "[" + r.TypeString(e.arg) + "]"
		r.errAt(r.Entities[e.site].File, r.nomonoSiteNode(e.site), cTypeNomono, from, to, r.cycleVia(edges, g, comp, e))
	}
	for owner := range bad {
		r.poisonFields(owner)
	}
}

func (r *Result) nomonoSiteNode(site EntityID) syntax.NodeID {
	e := &r.Entities[site]
	if e.Kind != EntType {
		return e.Node
	}
	t := r.tree(e.File)
	return t.View(typeDecl(t, e.Node).Body).AsDefinedBody().Type().ID()
}

func (r *Result) cycleVia(edges []typeEdge, g *instGraph, comp []int, bad typeEdge) string {
	c := comp[g.index[instVertex{bad.from.decl, bad.from.param}]]
	var names []string
	seen := map[EntityID]bool{}
	for _, e := range edges {
		d := e.from.decl
		if comp[g.index[instVertex{d, e.from.param}]] == c && !seen[d] && d != bad.from.decl {
			seen[d] = true
			names = append(names, r.Entities[d].Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return " through " + strings.Join(names, ", ")
}

func (r *Result) typeEdges(decl EntityID, edges []typeEdge) []typeEdge {
	info := r.typeDecl(decl)
	if len(info.Params) == 0 {
		return edges
	}
	if info.Form == FormDefined {
		return r.collectTypeEdges(decl, decl, info.Underlying, edges)
	}
	for _, f := range info.Fields {
		edges = r.collectTypeEdges(decl, f, r.Entities[f].Type, edges)
	}
	for _, v := range info.Variants {
		for _, t := range r.variant(v).Payload {
			edges = r.collectTypeEdges(decl, v, t, edges)
		}
	}
	return edges
}

func (r *Result) collectTypeEdges(decl, site EntityID, t TypeID, edges []typeEdge) []typeEdge {
	if t == 0 {
		return edges
	}
	tt := r.Types
	n := tt.Node(t)
	if n.Kind == KNamed && n.Ent != 0 {
		if callee := r.Entities[n.Ent]; callee.Kind == EntType {
			for j, arg := range n.Args {
				for i, p := range r.typeDecl(decl).Params {
					if pt := tt.Param(p); tt.Contains(arg, pt) {
						edges = append(edges, typeEdge{from: typeVertex{decl, i}, to: typeVertex{n.Ent, j}, grow: arg != pt, owner: decl, site: site, arg: arg})
					}
				}
			}
		}
	}
	edges = r.collectTypeEdges(decl, site, n.Elem, edges)
	for _, a := range n.Args {
		edges = r.collectTypeEdges(decl, site, a, edges)
	}
	return edges
}

func (r *Result) poisonFields(decl EntityID) {
	info := r.typeDecl(decl)
	if info.Form == FormDefined {
		info.Underlying = TyPoison
		return
	}
	for _, f := range info.Fields {
		r.Entities[f].Type = TyPoison
	}
	for _, v := range info.Variants {
		p := r.variant(v).Payload
		for i := range p {
			p[i] = TyPoison
		}
	}
}
