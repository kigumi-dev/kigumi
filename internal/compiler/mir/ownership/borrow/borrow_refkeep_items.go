package borrow

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// boundFn is a callable a call passes: a fn item or closure literal, with what its own witnesses or captures hold; a
// method value takes its first lead parameters (the receiver) from the value instead of the call.
type boundFn struct {
	ent  sem.EntityID
	sub  fnBind
	lead int
}

// fnBind maps a callee's argument slot (keptSlots' slot) to the callables that argument may be; a missing slot is unknown.
type fnBind map[int][]boundFn

func (b fnBind) key() string {
	var parts []string
	for j, fns := range b {
		var each []string
		for _, f := range fns {
			each = append(each, fmt.Sprintf("%d/%d{%s}", f.ent, f.lead, f.sub.key()))
		}
		slices.Sort(each)
		parts = append(parts, fmt.Sprint(j, each))
	}
	slices.Sort(parts)
	return strings.Join(parts, ";")
}

// ents is b's top-level callables by entity, the bind itemsIn and the plain kept summaries read.
func (b fnBind) ents() map[int][]sem.EntityID {
	var out map[int][]sem.EntityID
	for j, fns := range b {
		if out == nil {
			out = map[int][]sem.EntityID{}
		}
		for _, f := range fns {
			out[j] = append(out[j], f.ent)
		}
	}
	return out
}

func (b fnBind) with(o fnBind) fnBind {
	out := maps.Clone(b)
	for j, fns := range o {
		if out == nil {
			out = fnBind{}
		}
		out[j] = fns
	}
	return out
}

// defView is how refItems reads a body: the definitions of a local, its parameters, and what a parameter is bound to.
type defView struct {
	defs   func(midir.LocalID) []*midir.Inst
	params []midir.LocalID
	param  func(midir.LocalID) []boundFn
}

// siteView reads the body being checked, its own witnesses bound to what the program instantiates them with.
func (c *borrowCheck) siteView() defView {
	n := len(c.declaredParams(c.f.Ent))
	return defView{defs: c.defsOf, params: c.f.Params, param: func(l midir.LocalID) []boundFn { return c.ownWitness(c.f, n, l) }}
}

func (c *borrowCheck) keptView(k *keptCtx) defView {
	n := len(c.declaredParams(k.g.Ent))
	return defView{defs: func(l midir.LocalID) []*midir.Inst { return k.defs[l] }, params: k.g.Params, param: func(l midir.LocalID) []boundFn {
		if i := k.slot(l); i >= 0 && k.refs != nil {
			if fns, ok := k.refs.bind[i]; ok {
				return fns
			}
		}
		return c.ownWitness(k.g, n, l)
	}}
}

// ownWitness is witnessSet for a witness parameter l of fn body g, nil for any other local.
func (c *borrowCheck) ownWitness(g *midir.Func, n int, l midir.LocalID) []boundFn {
	if c.p.R.Entity(g.Ent).Kind != sem.EntFn || !slices.Contains(g.Params, l) {
		return nil
	}
	if w := slices.Index(c.hiddenLocals(g.Ent), g.Locals[l].Ent); w >= 0 && g.Locals[l].Ent != 0 {
		return c.witnessSet(g.Ent, w)
	}
	return nil
}

// refItems are the callables l may hold through copies, fn items, bound witnesses and closure literals, a parameter's
// binding joined with what the body assigns it; nil if any value it can hold is something else.
func (c *borrowCheck) refItems(v defView, l midir.LocalID, depth int) []boundFn {
	defs := v.defs(l)
	out := []boundFn{}
	if slices.Contains(v.params, l) {
		bound := v.param(l)
		if bound == nil || len(defs) == 0 {
			return bound
		}
		out = append(out, bound...)
	}
	if depth == 0 || len(defs) == 0 {
		return nil
	}
	for _, d := range defs {
		var fns []boundFn
		form := d.Form()
		switch {
		case d.Op == midir.OpFnItem:
			fns = []boundFn{{ent: d.Ent}}
		case d.Op == midir.OpBind, form == midir.FormClosure && c.p.R.Entity(d.Ent).Kind == sem.EntClosure:
			fns = []boundFn{{ent: d.Ent, sub: c.hiddenBind(v, d.Ent, d.Args, depth-1)}}
		case form == midir.FormMethod:
			fns = []boundFn{{ent: d.Ent, lead: 1}}
		case (form == midir.FormApplied || form == midir.FormWitness) && len(d.Args) == 2:
			inner := c.refItems(v, d.Args[0], depth-1)
			if inner != nil {
				fns = []boundFn{}
			}
			for _, f := range inner {
				f.lead = 1
				fns = append(fns, f)
			}
		case (d.Op == midir.OpCopy || d.Op == midir.OpMove || d.Op == midir.OpAlias) && len(d.Args) > 0:
			fns = c.refItems(v, d.Args[0], depth-1)
		}
		if fns == nil {
			return nil
		}
		out = append(out, fns...)
	}
	return out
}

// hiddenBind binds the parameters ent takes after its declared ones (a fn's witnesses, a closure's captures) to args.
func (c *borrowCheck) hiddenBind(v defView, ent sem.EntityID, args []midir.LocalID, depth int) fnBind {
	var out fnBind
	n := len(c.declaredParams(ent))
	for i, a := range args {
		if fns := c.refItems(v, a, depth); fns != nil {
			if out == nil {
				out = fnBind{}
			}
			out[n+i] = fns
		}
	}
	return out
}

// refBind binds each callable argument of a call to e, a declared callable parameter or a witness, that v resolves;
// arg is slot j's operand, absent for a slot the call does not pass (a method value's receiver).
func (c *borrowCheck) refBind(v defView, e sem.EntityID, arg func(int) (midir.LocalID, bool), types func(midir.LocalID) sem.TypeID) fnBind {
	var out fnBind
	params := len(c.declaredParams(e))
	for j := range params + len(c.hiddenLocals(e)) {
		a, ok := arg(j)
		if !ok || j < params && c.fnResult(types(a)) == 0 {
			continue
		}
		if fns := c.refItems(v, a, 8); fns != nil {
			if out == nil {
				out = fnBind{}
			}
			out[j] = fns
		}
	}
	return out
}

func argsAt(args []midir.LocalID) func(int) (midir.LocalID, bool) {
	return func(j int) (midir.LocalID, bool) {
		if j >= len(args) {
			return 0, false
		}
		return args[j], true
	}
}

func (c *borrowCheck) refBindSite(e sem.EntityID, args []midir.LocalID) fnBind {
	return c.refBind(c.siteView(), e, argsAt(args), func(a midir.LocalID) sem.TypeID { return c.f.Locals[a].Type })
}

func (c *borrowCheck) refBindIn(k *keptCtx, e sem.EntityID, args []midir.LocalID) fnBind {
	return c.refBind(c.keptView(k), e, argsAt(args), func(a midir.LocalID) sem.TypeID { return k.g.Locals[a].Type })
}
