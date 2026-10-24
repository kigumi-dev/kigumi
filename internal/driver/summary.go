package driver

import (
	"fmt"
	"strings"

	"kigumi/internal/schema"
)

// Summary is the build summary `kigumi build` prints after a successful build:
// the policy it ran under and what that policy let through.
type Summary struct {
	Policy         BuildPolicy
	Dependencies   []string
	LocalOverrides []string
	HostInputs     []string
	// Resources is ResourceReport's per-root lines; always reported,
	// strict profiles additionally gate the build on these.
	Resources []string
	// Capabilities is the reason graph's build-output half: the public
	// vocabulary of runtime features this build's reachable code draws on.
	Capabilities []string
	// ReproducibleChecked is true once --reproducible ran both builds; a mismatch
	// stops the build first, so Reproducible is then always true.
	ReproducibleChecked bool
	Reproducible        bool
	DeriveProvenance    []schema.GeneratedArtifact
	// Linking and Size are each build's resolved C library form and size settings, prefixed by artifact name in a build program.
	Linking []string
	Size    []string
}

func (s Summary) Render() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "build summary:\n  policy: %s\n", s.Policy.String())
	sb.WriteString("  dependencies: ")
	writeListOrNone(&sb, s.Dependencies)
	sb.WriteString("  local overrides: ")
	writeListOrNone(&sb, s.LocalOverrides)
	sb.WriteString("  host inputs: ")
	writeListOrNone(&sb, s.HostInputs)
	sb.WriteString("  resources: ")
	writeListOrNone(&sb, s.Resources)
	sb.WriteString("  capabilities: ")
	writeListOrNone(&sb, s.Capabilities)
	writeIfAny(&sb, "  linking: ", s.Linking)
	writeIfAny(&sb, "  size: ", s.Size)
	sb.WriteString("  derive generators: ")
	writeListOrNone(&sb, deriveProvenanceLines(s.DeriveProvenance))
	sb.WriteString("  build isolation: trusted host (not sandboxed)\n")
	sb.WriteString("  reproducibility: ")
	switch {
	case s.ReproducibleChecked && s.Reproducible:
		sb.WriteString("verified (two independent builds produced identical output)\n")
	case s.ReproducibleChecked:
		sb.WriteString("failed (two independent builds disagreed)\n")
	default:
		sb.WriteString("not verified (pass --reproducible to verify)\n")
	}
	return sb.String()
}

func deriveProvenanceLines(items []schema.GeneratedArtifact) []string {
	out := make([]string, len(items))
	for i, g := range items {
		name := ""
		if g.Origin != nil {
			name = g.Origin.Name
		}
		out[i] = fmt.Sprintf("%s on %s (%s)", g.Generator, name, short(g.ContentHash))
	}
	return out
}

func writeIfAny(sb *strings.Builder, label string, items []string) {
	if len(items) > 0 {
		sb.WriteString(label)
		writeListOrNone(sb, items)
	}
}

func writeListOrNone(sb *strings.Builder, items []string) {
	if len(items) == 0 {
		sb.WriteString("none\n")
		return
	}
	sb.WriteString(strings.Join(items, ", "))
	sb.WriteString("\n")
}
