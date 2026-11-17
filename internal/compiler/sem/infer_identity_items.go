package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// identityItems are the function items call a returns when its callee can only hand back one of its arguments: the
// declared result is a bare type parameter no interface bound could build, and each argument of that type is bare.
func (c *checker) identityItems(a syntax.NodeID, depth int) []EntityID {
	call, ok := c.info.Calls[a]
	if !ok || call.Kind != CallFn || call.Plan == nil || depth == 0 {
		return nil
	}
	tt := c.r.Types
	sig := tt.Node(c.r.Fn(call.Callee).Sig)
	if tt.Kind(sig.Elem) != KParam {
		return nil
	}
	g := tt.Node(sig.Elem).Ent
	if slices.ContainsFunc(c.r.typeParam(g).Constraints, func(con Constraint) bool { return con.Kind == CIface }) {
		return nil
	}
	args := call.Plan.placed()
	var items []EntityID
	for i, pt := range sig.Args {
		if !slices.Contains(tt.paramsIn(pt, nil), g) {
			continue
		}
		if tt.Kind(pt) != KParam || i >= len(args) || args[i] == 0 {
			return nil
		}
		got := c.fnItemsOf(args[i], depth-1)
		if len(got) == 0 {
			return nil
		}
		items = append(items, got...)
	}
	return items
}
