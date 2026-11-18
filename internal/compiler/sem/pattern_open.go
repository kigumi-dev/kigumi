package sem

import (
	"strconv"

	"kigumi/internal/compiler/syntax"
)

// openTarget gives each type argument of a type test's target that no enclosing generic binds a parameter of
// its own, read from the box at runtime: two tests of one type then never share an argument.
func (c *checker) openTarget(p syntax.NodeID, target TypeID) TypeID {
	tt := c.r.Types
	n := tt.Node(target)
	if n.Kind != KNamed || len(n.Args) == 0 {
		return target
	}
	opens, ok := c.info.Opens[p]
	if !ok {
		opens = make([]EntityID, len(n.Args))
		for i, a := range n.Args {
			if an := tt.Node(a); an.Kind == KParam && !c.r.typeParam(an.Ent).IsLifetime && !c.paramInScope(an.Ent) {
				opens[i] = c.openParam(p, an.Ent)
			}
		}
		if c.info.Opens == nil {
			c.info.Opens = map[syntax.NodeID][]EntityID{}
		}
		c.info.Opens[p] = opens
	}
	args := append([]TypeID(nil), n.Args...)
	for i, o := range opens {
		if o != 0 {
			args[i] = tt.Param(o)
		}
	}
	return tt.Named(n.Ent, args)
}

func (c *checker) paramInScope(p EntityID) bool {
	for ent := c.fn; ent != 0; ent = c.r.Entities[ent].Parent {
		switch c.r.Entities[ent].Kind {
		case EntFn:
			info := c.r.Fn(ent)
			for _, q := range append(append([]EntityID(nil), info.TypeParams...), info.RecvParams...) {
				if q == p {
					return true
				}
			}
		case EntClosure:
		default:
			return false
		}
	}
	return false
}

// openParam copies the declared parameter decl for one test, with a hidden local the test fills.
func (c *checker) openParam(p syntax.NodeID, decl EntityID) EntityID {
	info := *c.r.typeParam(decl)
	info.Opened = true
	e := c.r.Entities[decl]
	e.File, e.Node, e.Parent = c.f, p, c.fn
	e.Detail = c.r.addTypeParam(info)
	param := c.r.newEntity(e)
	ty := c.r.Types.RtType()
	if info.IsConst {
		ty = info.ConstType
	}
	name := openLocalName(c.r, param)
	local := c.r.newEntity(Entity{Kind: EntLocal, Name: name, Pkg: c.pkg, File: c.f, Node: p, Parent: c.fn, Type: ty})
	c.r.Entities[local].Detail = c.r.addLocal(LocalInfo{Scope: c.scope, Opens: param})
	c.r.define(c.scope, name, Binding{Ent: local})
	c.r.typeParam(param).Local = local
	c.opens = append(c.opens, param)
	return param
}

// openLocalName is unique per test: a const one is found as constParamLocalName finds a declared one.
func openLocalName(r *Result, param EntityID) string {
	if r.typeParam(param).IsConst {
		return constParamLocalName(r, param)
	}
	return "$type:" + r.Entities[param].Name + "#" + strconv.Itoa(int(param))
}

// bindsNames reports whether scope binds a name the source can write, past the hidden locals of opened parameters.
func (r *Result) bindsNames(scope ScopeID) bool {
	for _, b := range r.Scopes[scope].Names {
		if b.Ent == 0 || r.Entities[b.Ent].Kind != EntLocal || r.local(b.Ent).Opens == 0 {
			return true
		}
	}
	return false
}

// openSite is where the test that opened param sits, or "" for a declared parameter.
func (r *Result) openSite(param EntityID) string {
	if r.Entities[param].Kind != EntTypeParam || !r.typeParam(param).Opened {
		return ""
	}
	e := r.Entities[param]
	f := r.Files[e.File].Tree.File
	at := r.Files[e.File].span(e.Node).Start
	return f.Name + ":" + strconv.Itoa(f.Line(at)) + ":" + strconv.Itoa(f.Column(at))
}

// OpenedLocal is the hidden local a type test fills for the parameter it opened.
func (r *Result) OpenedLocal(param EntityID) (EntityID, bool) {
	if r.Entities[param].Kind != EntTypeParam {
		return 0, false
	}
	info := r.typeParam(param)
	return info.Local, info.Opened
}
