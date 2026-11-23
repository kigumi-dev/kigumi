package driver

import (
	"fmt"
	"sort"
	"strings"

	"kigumi/internal/toolchain/policy"
)

// checkUnusedReplace warns about a Replace naming a module the selected graph does not contain; it resolves nothing, so it is usually a misspelling or a leftover. A release build refuses every Replace anyway.
func (m *Module) checkUnusedReplace() {
	used := map[string]bool{}
	for key, r := range m.Selected {
		if _, ok := m.local.Replaces[key]; ok {
			used[key] = true
		}
		if r.Alias != "" {
			// A plain-name Replace applies to every alias of that module.
			if _, ok := m.local.Replaces[r.Name]; ok {
				used[r.Name] = true
			}
		}
	}
	var names []string
	for name := range m.local.Replaces {
		if !used[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		hint := ""
		for _, key := range m.SelectedKeys() {
			if strings.EqualFold(key, name) {
				hint = fmt.Sprintf("; did you mean %q?", key)
			}
		}
		m.UnusedReplaces = append(m.UnusedReplaces, policy.UnusedLocalReplaceViolation(name, hint, m.local.Provenance[name]))
	}
}

// SelectedKeys lists the effective dependency keys in a stable order.
func (m *Module) SelectedKeys() []string {
	keys := make([]string, 0, len(m.Selected))
	for key := range m.Selected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
