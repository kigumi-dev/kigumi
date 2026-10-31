package sem

import (
	"slices"
	"strings"
)

func splitContracts(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ";")
}

func joinContracts(set map[string]bool) string {
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return strings.Join(ids, ";")
}
