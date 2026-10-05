package explain

import (
	"fmt"
	"io"

	"kigumi/internal/cli/shared"
	"kigumi/internal/cliutil"
	"kigumi/internal/driver"
	"kigumi/internal/errors"

	"github.com/spf13/cobra"
)

func explainCapabilities(cmd *cobra.Command, target string) error {
	m, err := shared.Load(cmd, target, false, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	a, err := driver.Analyze(m)
	if err != nil {
		return errors.WrapErr(err, "analyze module")
	}
	if a.Prog == nil {
		return &cliutil.UsageError{Err: errors.NewErr("module has diagnostics; fix them first")}
	}
	printCapabilities(cmd.OutOrStdout(), driver.CapabilityReasonGraph(a))
	return nil
}

func printCapabilities(out io.Writer, graph []driver.CapabilityPath) {
	if len(graph) == 0 {
		fmt.Fprint(out, "no named capability in this closed vocabulary\n")
		return
	}
	for _, cp := range graph {
		fmt.Fprintf(out, "%s\n", cp.Name)
		for _, hop := range cp.Path {
			fmt.Fprintf(out, "  %s\n", hop)
		}
	}
}
