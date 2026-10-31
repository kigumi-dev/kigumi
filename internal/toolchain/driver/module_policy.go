package driver

import (
	"fmt"
	"sort"

	"kigumi/internal/toolchain/policy"
)

// LocalOverrides lists the mod.local.kg replacements in effect, sorted by
// name, for the build summary.
func (m *Module) LocalOverrides() []string {
	names := make([]string, 0, len(m.local.Replaces))
	for name := range m.local.Replaces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Dependencies lists what minimal version selection resolved, sorted by
// name, for the build summary.
func (m *Module) Dependencies() []string {
	names := make([]string, 0, len(m.Selected))
	for name := range m.Selected {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = fmt.Sprintf("%s %s", name, m.Selected[name].Version)
	}
	return out
}

// checkLocalReplace records a release-local-replace violation for every
// mod.local.kg entry; a policy that denies local overrides fails the load outright.
func (m *Module) checkLocalReplace(pol policy.BuildPolicy) error {
	names := make([]string, 0, len(m.local.Replaces))
	for name := range m.local.Replaces {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		v := policy.LocalReplaceViolation(name, m.local.Replaces[name], m.local.Provenance[name])
		if pol.LocalOverride == policy.LocalOverrideDeny {
			return policy.Error(v)
		}
		m.Violations = append(m.Violations, v)
	}
	return nil
}
