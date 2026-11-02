package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// callResult is a call's result type and the type expected of it, 0 when nothing is expected.
type callResult struct{ ret, want TypeID }

// erasedWant is what the call's expected result makes of param (an open variable stands for its Fn bound) when that
// holds a fn type with a plain reference parameter, which no argument naming a lifetime can infer.
func (c *checker) erasedWant(param TypeID, res callResult) TypeID {
	if res.want == 0 || !c.r.Types.ContainsVar(c.vars.resolve(param)) {
		return 0
	}
	mark := c.vars.mark()
	var ft TypeID
	if c.vars.unify(res.ret, res.want) {
		ft = c.vars.resolve(param)
		if i, ok := c.vars.index(ft); ok && c.vars.hint[i] != 0 {
			ft = c.vars.resolve(c.vars.hint[i])
		}
	}
	c.vars.rollback(mark)
	if ft == 0 || !c.r.Types.plainRefParam(ft) {
		return 0
	}
	return ft
}

func (tt *TypeTable) plainRefParam(t TypeID) bool {
	n := tt.Node(t)
	if n.Kind == KFn && slices.ContainsFunc(n.Args, func(p TypeID) bool { return tt.Kind(p) == KRef && tt.RefLifetime(p) == 0 }) {
		return true
	}
	return n.Elem != 0 && tt.plainRefParam(n.Elem) || slices.ContainsFunc(n.Args, tt.plainRefParam)
}

func (c *checker) expectErasedCall(a syntax.NodeID, param TypeID, res callResult) {
	if k := c.t.Kind(a); k != syntax.CallExpr && k != syntax.WsCallExpr {
		return
	}
	if ft := c.erasedWant(param, res); ft != 0 && !c.r.Types.ContainsVar(ft) {
		c.vars.unify(param, ft)
	}
}

// erasedItemWant lets generic fn item a, of type got, lose a named reference lifetime by the coercion a direct use
// takes, not by unification, which keeps `&'a T` and `&T` parameters apart.
func (c *checker) erasedItemWant(a syntax.NodeID, got, param TypeID, res callResult) TypeID {
	if _, ok := c.genericFnItemCallable(a); !ok {
		return 0
	}
	hint := c.vars.fnHint(param)
	ft := c.erasedWant(param, res)
	if c.r.Types.Kind(ft) != KFn || c.vars.unifiable(got, ft) {
		return 0
	}
	mark := c.vars.mark()
	fits := c.fitsErased(got, ft, true)
	c.vars.rollback(mark)
	if !fits || !c.vars.unify(param, ft) {
		return 0
	}
	if hint != 0 {
		c.vars.unifyIgnoringEscaping(hint, ft)
	}
	return ft
}
