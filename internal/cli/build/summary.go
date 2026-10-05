package build

import (
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/driver"
)

func buildSummary(cmd *cobra.Command, m *driver.Module, policy driver.BuildPolicy, reproducible bool) driver.Summary {
	var inputs []string
	if path, ambient := shared.AmbientStd(cmd); ambient {
		inputs = append(inputs, "std: "+path+" (from $KIGUMI_STD)")
	} else {
		inputs = append(inputs, "std: "+shared.StdRoot(cmd))
	}
	if flags, ambient := shared.AmbientCFlags(); ambient {
		inputs = append(inputs, "cflags: "+strings.Join(flags, " ")+" (from $KIGUMI_CFLAGS)")
	}
	return driver.Summary{
		Policy:              policy,
		Dependencies:        m.Dependencies(),
		LocalOverrides:      m.LocalOverrides(),
		HostInputs:          inputs,
		Resources:           driver.ResourceReport(m.LastAnalysis()),
		ReproducibleChecked: reproducible,
		Reproducible:        reproducible,
	}
}
