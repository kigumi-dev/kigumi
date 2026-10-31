package llgen

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// unwindAggSlots points each counted local's unwind slot at the local, tagged so the sweep calls its drop helper instead of a release.
func (e *emitter) unwindAggSlots(sb *strings.Builder) {
	u := e.unwind
	if u == nil {
		return
	}
	for _, l := range slices.Sorted(maps.Keys(u.agg)) {
		k := u.slot[l]
		addr, tagged, p, q := e.newTmp(), e.newTmp(), e.newTmp(), e.newTmp()
		fmt.Fprintf(sb, "  %s = ptrtoint ptr %s to i64\n  %s = or i64 %s, 1\n", addr, aggPtr(l), tagged, addr)
		fmt.Fprintf(sb, "  %s = getelementptr i64, ptr %%urec, i64 %d\n  store i64 %s, ptr %s\n", p, u.slotWord(k), tagged, p)
		fmt.Fprintf(sb, "  %s = getelementptr i64, ptr %%urec, i64 %d\n  store ptr %s, ptr %s\n", q, u.slotWord(k+1), e.refHelper(u.agg[l], refDrop), q)
	}
}
