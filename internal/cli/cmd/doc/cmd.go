// Package doc implements `kigumi doc`: render the declarations of a module
// or of std as Markdown.
package doc

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc [-o <dir>] [--all] [--html] <module-root|std>",
		Short: "Render the documentation of a module (or of std) as Markdown, or as a static HTML site",
		Args:  cobra.ExactArgs(1),
		RunE:  session.RunE(parseOptions, run),
	}
	cmd.Flags().StringP("output", "o", "", "directory for one .md per package plus README.md (default: stdout)")
	cmd.Flags().Bool("all", false, "include declarations without pub")
	cmd.Flags().Bool("html", false, "write a static HTML site (index.html and one page per package) instead of Markdown")
	cmd.AddCommand(examplesCmd())
	return cmd
}

// examplesCmd renders examples/catalog.json, the file the example runner itself reads, as a Markdown or HTML catalog page.
func examplesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "examples [-o <dir>] [--html] [<examples-root>]",
		Short: "Render the examples/ catalog (what each example shows, its oracle, its backends) as Markdown or HTML",
		Args:  cobra.MaximumNArgs(1),
		RunE:  session.RunE(parseExamplesOptions, runExamples),
	}
	cmd.Flags().StringP("output", "o", "", "directory for the catalog page (default: stdout)")
	cmd.Flags().Bool("html", false, "write the catalog as an HTML page instead of Markdown")
	return cmd
}
