// Package emit implements the binding emitters that derive their output
// from the Schema IR: `kigumi emit-schema` and `kigumi emit-c-header`.
package emit

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

// NewSchema is `kigumi emit-schema`: the machine-readable schema document a
// binding generator consumes.
func NewSchema() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "emit-schema [--attr <pkg.name>] [--allow-errors] [-o <path>] <module-root>",
		Short: "Emit the module's versioned Schema IR as the canonical binding schema JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  session.RunE(parseSchemaOptions, runSchema),
	}
	cmd.Flags().String("attr", "", "keep only declarations carrying this attribute")
	cmd.Flags().Bool("allow-errors", false, "emit the declarations even when the module has type errors")
	cmd.Flags().StringP("output", "o", "", "write the schema to this file (default: stdout)")
	return cmd
}

// NewCHeader is `kigumi emit-c-header`: C declarations for the ABI boundary.
func NewCHeader() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "emit-c-header [--format text|json] [--guard <macro>] [-o <path>] <module-root>",
		Short: "Emit a C header declaring the module's extern/export(C) types and functions",
		Args:  cobra.ExactArgs(1),
		RunE:  session.RunE(parseHeaderOptions, runHeader),
	}
	cmd.Flags().String("format", "text", "output format: text (the header) or json (the header plus diagnostics)")
	cmd.Flags().String("guard", "", "include guard macro (default: derived from the first scope package)")
	cmd.Flags().StringP("output", "o", "", "write the output to this file (default: stdout)")
	return cmd
}
