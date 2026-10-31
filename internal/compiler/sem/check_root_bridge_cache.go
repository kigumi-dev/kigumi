package sem

import (
	"slices"
	"strconv"
	"strings"
)

type rootForwardKey struct {
	callee EntityID
	bind   string
}

// Only siblings share a caller, ancestors and reachability snapshot, so this cache stays within one frame.
type rootForwardCache map[rootForwardKey]int

func rootBindingKey(subst map[EntityID][]EntityID) string {
	params := make([]EntityID, 0, len(subst))
	for p := range subst {
		params = append(params, p)
	}
	slices.Sort(params)
	var key strings.Builder
	for _, p := range params {
		key.WriteString(strconv.FormatUint(uint64(p), 10))
		key.WriteByte(':')
		values := slices.Clone(subst[p])
		slices.Sort(values)
		for _, e := range slices.Compact(values) {
			key.WriteString(strconv.FormatUint(uint64(e), 10))
			key.WriteByte(',')
		}
		key.WriteByte(';')
	}
	return key.String()
}
