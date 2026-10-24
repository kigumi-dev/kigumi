package build

import (
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/driver"
	"kigumi/internal/schema"
)

func buildSummary(cmd *cobra.Command, m *driver.Module, policy driver.BuildPolicy, reproducible bool, tools []string, reports map[string]driver.BuildReport) driver.Summary {
	var inputs []string
	if path, ambient := shared.AmbientStd(cmd); ambient {
		inputs = append(inputs, "std: "+path+" (from $KIGUMI_STD)")
	} else {
		inputs = append(inputs, "std: "+shared.StdRoot(cmd))
	}
	if flags, ambient := shared.AmbientCFlags(); ambient {
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
		Policy:              policy,
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
