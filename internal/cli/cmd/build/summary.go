package build

import (
	"strings"

	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/contract/schema"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/policy"
)

func buildSummary(s session.Session, m *driver.Module, pol policy.BuildPolicy, reproducible bool, tools []string, reports map[string]driver.BuildReport) driver.Summary {
	var inputs []string
	if path, ambient := s.AmbientStd(); ambient {
		inputs = append(inputs, "std: "+path+" (from $KIGUMI_STD)")
	} else {
		inputs = append(inputs, "std: "+s.StdRoot())
	}
	if flags, ambient := session.AmbientCFlags(); ambient {
		inputs = append(inputs, "cflags: "+strings.Join(flags, " ")+" (from $KIGUMI_CFLAGS)")
	}
	var provenance []schema.GeneratedArtifact
	if a := m.LastAnalysis(); a != nil {
		provenance = a.Provenance
	}
	linking, size := shapeLines(reports)
	return driver.Summary{
		Linking:             linking,
		Size:                size,
		Policy:              pol,
		Dependencies:        m.Dependencies(),
		LocalOverrides:      m.LocalOverrides(),
		HostInputs:          inputs,
		Resources:           driver.ResourceReport(m.LastAnalysis()),
		Capabilities:        append(driver.Capabilities(m.LastAnalysis()), tools...),
		ReproducibleChecked: reproducible,
		Reproducible:        reproducible,
		DeriveProvenance:    provenance,
	}
}
