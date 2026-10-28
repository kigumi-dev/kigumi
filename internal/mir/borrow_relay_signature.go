package mir

import (
	"slices"

	"kigumi/internal/sem"
)

func (c *borrowCheck) calleeKeepsBorrow(t sem.TypeID) bool {
	params, known := c.calleeParams(t)
	return !known || slices.ContainsFunc(params, c.paramKeepsBorrow)
}

func (c *borrowCheck) paramKeepsBorrow(t sem.TypeID) bool {
	tt := c.p.R.Types
	if n := tt.Node(t); n.Kind == sem.KRef && c.declaredLifetime(t) == 0 {
		if e := tt.Node(n.Elem); !tt.RefMut(t) && e.Kind == sem.KParam && !c.p.R.IsCopy(n.Elem) && len(c.p.R.TypeParam(e.Ent).Constraints) == 0 {
			return false
		}
		t = n.Elem
	}
	return tt.Kind(t) == sem.KPoison || tt.Kind(t) == sem.KVar || c.holdsBorrow(t) || c.holdsDeferred(t) || c.namesTypeParam(t)
}
