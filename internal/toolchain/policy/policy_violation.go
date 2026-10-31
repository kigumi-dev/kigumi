package policy

import (
	"fmt"
	"sort"
	"strings"
)

// Violation is one release-policy finding: a stable ID so Development and Release
// render the same cause and help text, differing only in warning vs. error.
type Violation struct {
	ID      string
	Subject string // one-line summary: "<thing> is <condition>"
	Detail  []KV   // ordered key: value lines (provenance, what was rejected)
	Policy  string // the "release policy: ..." line's reason, release wording
	Help    string
}

// KV is one indented "key: value" line under a rendered Violation.
type KV struct{ Key, Value string }

func Detail(key, value string) KV { return KV{Key: key, Value: value} }

// severity reports whether id is an error under p, given which policy
// field governs it. Unknown ids never fail a build.
func (p BuildPolicy) severity(id string) (isError bool) {
	switch id {
	case "release-local-replace":
		return p.LocalOverride == LocalOverrideDeny
	case "release-unlocked-script":
		return p.Dependency == DependencyLocked
	case "release-ambient-std", "release-ambient-cflags":
		return p.Ambient == AmbientDeny
	case "release-native-link-unresolved":
		return p.Dependency == DependencyLocked
	case "release-native-link-identity-unknown":
		return p.IsRelease()
	case "release-unverified-target":
		return p.IsRelease()
	default:
		return false
	}
}

// Report is a release predicate's findings, split into what Development
// warns about and what Release refuses to build.
type Report struct {
	Warnings []Violation
	Errors   []Violation
}

// HasErrors reports whether the report must stop the build.
func (r Report) HasErrors() bool { return len(r.Errors) > 0 }

// Evaluate classifies collected violations and extra
// CLI-collected ones, under policy: Development warns, Release fails the build.
func Evaluate(violations []Violation, policy BuildPolicy, extra ...Violation) Report {
	var r Report
	all := append(append([]Violation{}, violations...), extra...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	for _, v := range all {
		if policy.severity(v.ID) {
			r.Errors = append(r.Errors, v)
		} else {
			r.Warnings = append(r.Warnings, v)
		}
	}
	return r
}

// Render formats every violation (warnings first, then errors) followed by a
// release-readiness summary.
func (r Report) Render() string {
	var sb strings.Builder
	for _, v := range r.Warnings {
		renderViolation(&sb, "warning", v)
	}
	for _, v := range r.Errors {
		renderViolation(&sb, "error", v)
	}
	switch {
	case len(r.Errors) > 0:
		fmt.Fprintf(&sb, "release readiness: %d error(s), %d warning(s); build refused\n", len(r.Errors), len(r.Warnings))
	case len(r.Warnings) > 0:
		fmt.Fprintf(&sb, "release readiness: %d warning(s); would fail with --release\n", len(r.Warnings))
	}
	return sb.String()
}

func renderViolation(sb *strings.Builder, severity string, v Violation) {
	fmt.Fprintf(sb, "%s[%s]: %s\n", severity, v.ID, v.Subject)
	for _, d := range v.Detail {
		fmt.Fprintf(sb, "  %s: %s\n", d.Key, d.Value)
	}
	if v.Policy != "" {
		fmt.Fprintf(sb, "  release policy: %s\n", v.Policy)
	}
	if v.Help != "" {
		fmt.Fprintf(sb, "  help: %s\n", v.Help)
	}
}
