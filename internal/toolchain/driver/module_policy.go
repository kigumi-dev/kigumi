package driver

import (
	"fmt"
	"sort"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/native/link"
	"kigumi/internal/toolchain/policy"
)

// LocalReplaceDir reports the directory a Replace record sent name to, so
// callers can tell a developer's checkout apart from the module cache.
func (m *Module) LocalReplaceDir(name string) (string, bool) {
	p, ok := m.local.Replaces[name]
	return p, ok
}

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

// LinkIdentityViolations lists a violation for every
// Link in m that link.ResolveLinkIdentity could not find against t.
func LinkIdentityViolations(m *Module, t buildconfig.Target) []policy.Violation {
	var vs []policy.Violation
	for _, l := range m.Links {
		if l.Library == "" {
			continue
		}
		if _, ok := link.ResolveLinkIdentity(m.compiler, link.Library{Name: l.Library, Search: l.Search}, t); !ok {
			vs = append(vs, policy.NativeLinkIdentityUnknownViolation(m.Name, l.Library))
		}
	}
	return vs
}
