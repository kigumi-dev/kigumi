package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// each visits the entries that are set.
func (v *cowVec[T]) each(fn func(l midir.LocalID, x T)) {
	var zero T
	for t, nd := range v.top {
		if nd == nil {
			continue
		}
		for c, lf := range nd.leaf {
			if lf == nil {
				continue
			}
			for i, x := range lf.v {
				if x != zero {
					fn(midir.LocalID(t*cowSpan+c*cowLeafSize+i), x)
				}
			}
		}
	}
}
