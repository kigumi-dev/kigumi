package sem

import (
	"cmp"

	"kigumi/internal/compiler/syntax"
)

// mismatch reports a failed coercion with the most specific wording.
func (c *checker) mismatch(n syntax.NodeID, want, got TypeID) {
	tt := c.r.Types
	want, got = c.vars.resolve(want), c.vars.resolve(got)
	if i, ok := c.vars.index(want); ok {
		if c.vars.pending[i] == 0 {
			return
		}
		want = TyI64
		if c.vars.pending[i] == 2 {
			want = TyF64
		}
	}
	if want == TyPoison || got == TyPoison {
		return
	}
	if want != got && c.sameButEscaping(want, got) {
		c.errAt(n, cEscapingNested, c.typeText(got), c.typeText(want))
		c.vars.poison(got)
		return
	}
	if tt.Kind(got) == KUntyped && !tt.IsNumeric(want) {
		got = c.adopt(c.literalNode(n), defaultOf(got))
	}
	if tt.Kind(want) == KIface && !c.r.objectSafe(tt.Node(want).Ent) {
		return
	}
	if tt.Kind(want) == KIface {
		if res := c.r.conforms(got, want, c.pkg); res.generic {
			c.errAt(n, cIfaceCoerceUnsat, got, want, res.missing)
			c.vars.poison(got)
			c.vars.poison(want)
			return
		}
	}
	if _, _, ok := tt.IsResult(got); ok {
		if _, _, ok := tt.IsResult(want); ok {
			c.errAt(n, cResultAlready, got)
			return
		}
	}
	if tt.Kind(want) == KIface && tt.Node(want).Ent == tt.errorEnt {
		if tt.Kind(got) == KParam && c.paramHasIface(got, want) {
			c.errAt(n, cErrorParamBox, got)
			return
		}
		c.errAt(n, cErrorNotConform, got)
		return
	}
	if call, ok := c.info.Calls[n]; ok && call.Kind == CallMethodValue && tt.Kind(want) != KFn && tt.Kind(want) != KClosure {
		c.errAt(n, cMethodNeedsCall, c.r.Entities[call.Callee].Name, c.r.Entities[call.Callee].Name)
		return
	}
	if tt.Kind(got) == KUntyped && tt.IsNumeric(want) {
		c.checkLiteralFits(c.literalNode(n), want)
		return
	}
	if c.closureMismatch(n, cTypeMismatch, want, got) {
		return
	}
	ws, gs := c.r.mismatchLabels(c.vars.literalLabel(want), c.vars.literalLabel(got))
	if n == c.wantAliasTarget {
		if alias, ok := c.r.aliasNameAt(c.wantAliasFile, c.wantAliasNode, want); ok {
			ws = alias
		}
	}
	c.errAtHelp(n, cmp.Or(usizeHelp(want, got), c.fallThroughHelp(n, want, got)), cTypeMismatch, ws, gs)
	c.vars.poison(got)
	c.vars.poison(want)
}

func (c *checker) fallThroughHelp(n syntax.NodeID, want, got TypeID) string {
	if got != TyUnit || c.t.Kind(n) != syntax.Block || want != c.vars.resolve(c.retType) {
		return ""
	}
	return "not every path ends in a value; finish the block with the value to produce, or `return` it on each path"
}
