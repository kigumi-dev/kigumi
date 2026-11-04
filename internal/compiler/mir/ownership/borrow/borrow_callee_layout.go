package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// binding is one set of operands a callable value binds around the arguments it is called with.
type binding struct {
	trails bool
	bound  []midir.LocalID
}

func bindingOf(d *midir.Inst, bound []midir.LocalID) binding {
	return binding{trails: midir.BoundTrails(d.Op), bound: bound}
}

// operand is what a body parameter of a callee origin gets: operand index of binding layer, or argument index of the
// call.
type operand struct {
	local midir.LocalID
	index int
	layer int
	bound bool
}

// operands lays out the body parameters of a call passing nargs arguments, innermost binding first.
func (o calleeOrigin) operands(nargs int) []operand {
	out := make([]operand, nargs)
	for k := range out {
		out[k] = operand{index: k}
	}
	for n, b := range o.layers {
		bound := make([]operand, len(b.bound))
		for j, l := range b.bound {
			bound[j] = operand{local: l, index: j, layer: n, bound: true}
		}
		out = midir.AppendValueParams(nil, b.trails, bound, out)
	}
	return out
}

func (o calleeOrigin) nbound() int {
	n := 0
	for _, b := range o.layers {
		n += len(b.bound)
	}
	return n
}

// params are the locals the body runs with when the call passes args.
func (o calleeOrigin) params(args []midir.LocalID) []midir.LocalID {
	out := make([]midir.LocalID, 0, len(args)+o.nbound())
	for _, x := range o.operands(len(args)) {
		if x.bound {
			out = append(out, x.local)
		} else {
			out = append(out, args[x.index])
		}
	}
	return out
}

func (o calleeOrigin) argParam(k int) int {
	for _, b := range o.layers {
		k = midir.ArgParam(b.trails, len(b.bound), k)
	}
	return k
}

// boundParam is the body parameter operand j of binding layer n lands on, -1 without a body.
func (o calleeOrigin) boundParam(n, j int) int {
	if o.body == nil {
		return -1
	}
	return slices.IndexFunc(o.operands(max(len(o.body.Params)-o.nbound(), 0)), func(x operand) bool {
		return x.bound && x.layer == n && x.index == j
	})
}

// argParams are the body parameters a call's arguments take.
func (o calleeOrigin) argParams() []int {
	var out []int
	if o.body == nil {
		return nil
	}
	for k := range max(len(o.body.Params)-o.nbound(), 0) {
		out = append(out, o.argParam(k))
	}
	return out
}

// boundLocals are the locals the origin binds to its body's parameters; the others are a call's, unknown here.
func (o calleeOrigin) boundLocals() paramLocals {
	if o.body == nil {
		return func(int) (midir.LocalID, bool) { return 0, false }
	}
	ops := o.operands(max(len(o.body.Params)-o.nbound(), 0))
	return func(p int) (midir.LocalID, bool) {
		if p < 0 || p >= len(ops) || !ops[p].bound {
			return 0, false
		}
		return ops[p].local, true
	}
}

// paramLocals is the local a call passes body parameter p, false when it is not known here.
type paramLocals func(p int) (midir.LocalID, bool)

func localsOf(args []midir.LocalID) paramLocals {
	return func(p int) (midir.LocalID, bool) {
		if p < 0 || p >= len(args) {
			return 0, false
		}
		return args[p], true
	}
}

// argDeclared is the index among o's declared parameters of the one argument k of a call lands on; false when it lands
// on a parameter that is not declared, as a closure's captures are not.
func (c *borrowCheck) argDeclared(o calleeOrigin, k int) (int, bool) {
	declared, j := c.declaredParams(o.ent), o.argParam(k)
	if o.body == nil {
		return j, j < len(declared)
	}
	if j >= len(o.body.Params) {
		return 0, false
	}
	i := slices.Index(declared, o.body.Locals[o.body.Params[j]].Ent)
	return i, i >= 0
}
