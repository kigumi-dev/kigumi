package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// linkSlots widens each slot's parameters to every type parameter a constraint ties them
// to: under `S: Sink[T]`, a witness call through S may store a T into the S place.
func (c *borrowCheck) linkSlots(info *sem.FnInfo, slots []callSlot) {
	group := map[sem.EntityID][]sem.EntityID{}
	for _, p := range info.TypeParams {
		for _, con := range c.p.R.TypeParam(p).Constraints {
			for _, q := range c.typeParamsOf(con.Type) {
				group[p] = append(group[p], q)
				group[q] = append(group[q], p)
			}
		}
	}
	if len(group) == 0 {
		return
	}
	for i := range slots {
		ps := append([]sem.EntityID(nil), slots[i].params...)
		for k := 0; k < len(ps); k++ {
			for _, q := range group[ps[k]] {
				if !slices.Contains(ps, q) {
					ps = append(ps, q)
				}
			}
		}
		slots[i].params = ps
	}
}
