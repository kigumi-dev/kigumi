package driver

import (
	"fmt"
	"sort"
	"strings"

	"kigumi/internal/toolchain/policy"
)

// checkUnusedReplace warns about a Replace naming a module the selected graph does not contain; it resolves nothing, so it is usually a misspelling or a leftover. A release build refuses every Replace anyway.
func (m *Module) checkUnusedReplace() {
	var names []string
	for name := range m.local.Replaces {
		if _, ok := m.Selected[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		hint := ""
		for sel := range m.Selected {
			if strings.EqualFold(sel, name) {
				hint = fmt.Sprintf("; did you mean %q?", sel)
			}
		}
		m.UnusedReplaces = append(m.UnusedReplaces, policy.UnusedLocalReplaceViolation(name, hint, m.local.Provenance[name]))
	}
}
