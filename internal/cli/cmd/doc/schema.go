package doc

import (
	"fmt"

	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/tooling/doc"
)

// SchemaCmd prints the module's declarations with their metadata
// attributes as JSON, the declaration query generators build on.
func SchemaCmd() *cobra.Command {
	var attr string
	var allowErrors bool
	var ir bool
	cmd := &cobra.Command{
		Use:   "schema [--attr <pkg.name>] [--ir] [--allow-errors] <module-root>",
		Short: "List declarations and their metadata attributes as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := shared.LoadWhole(cmd, args[0], true, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			res, err := driver.Check(m)
			if err != nil {
				return errors.WrapErr(err, "check module")
			}
			if res.HasErrors() {
				cmd.PrintErr(res.Render())
				if !allowErrors {
					return cliutil.Exit(1)
				}
			}
			if ir {
				tgt, err := shared.Target(cmd)
				if err != nil {
					return &cliutil.UsageError{Err: err}
				}
				target := tgt.Triple
				if target == "" {
					target = tgt.OS + "-" + tgt.Arch
				}
				out, err := doc.SchemaIRJSON(m, res, attr, target, tgt.Profile)
				if err != nil {
					return errors.WrapErr(err, "build schema IR")
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), doc.SchemaJSON(doc.Declarations(m, res, attr)))
			return nil
		},
	}
	cmd.Flags().StringVar(&attr, "attr", "", "keep only declarations carrying this attribute")
	cmd.Flags().BoolVar(&allowErrors, "allow-errors", false, "print the declarations even when the module has type errors (generators run before their output exists)")
	cmd.Flags().BoolVar(&ir, "ir", false, "print the versioned typed Schema IR instead of the legacy signature-string projection")
	return cmd
}
