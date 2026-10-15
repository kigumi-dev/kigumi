// Package check implements `kigumi check`: type-check a module.
package check

import (
	"fmt"

	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/cliutil"
	"kigumi/internal/driver"
	"kigumi/internal/errors"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <module-root|entry.kg>",
		Short: "Type-check a module and print its diagnostics",
		Args:  cobra.ExactArgs(1),
	}
	shared.ReleaseFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		policy := shared.Policy(cmd)
		m, err := shared.LoadSettled(cmd, args[0], false, cmd.ErrOrStderr(), policy)
		if err != nil {
			return err
		}
		extra := append(shared.ReleaseViolations(cmd, false), driver.LinkIdentityViolations(m, shared.CompilerOptions().Host)...)
		report := driver.EvaluateRelease(m, policy, extra...)
		fmt.Fprint(cmd.ErrOrStderr(), report.Render())
		if report.HasErrors() {
			return cliutil.Exit(1)
		}
		res, err := driver.CheckUnits(m)
		if err != nil {
			return errors.WrapErr(err, "check module")
		}
		fmt.Fprint(cmd.ErrOrStderr(), res.Render())
		if res.HasErrors() {
			return cliutil.Exit(1)
		}
		return nil
	}
	return cmd
}
