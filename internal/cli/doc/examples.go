package doc

import (
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/cliutil"
	"kigumi/internal/doc"
	"kigumi/internal/errors"
)

// ExamplesCmd renders examples/catalog.json, the file the example runner itself reads, as a Markdown or HTML catalog page.
func ExamplesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "examples [-o <dir>] [--html] [<examples-root>]",
		Short: "Render the examples/ catalog (what each example shows, its oracle, its backends) as Markdown or HTML",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "examples"
			if len(args) == 1 {
				root = args[0]
			}
			md, err := doc.ExamplesCatalogMarkdown(root)
			if err != nil {
				return errors.WrapErr(err, "load examples catalog")
			}
			outDir, _ := cmd.Flags().GetString("output")
			if asHTML, _ := cmd.Flags().GetBool("html"); asHTML {
				if outDir == "" {
					return &cliutil.UsageError{Err: errors.NewErrf("--html needs -o <dir>")}
				}
				site := doc.HTML(map[string]string{"examples": md}, "Examples")
				files := make(map[string][]byte, len(site))
				for name, text := range site {
					files[name] = []byte(text)
				}
				if err := doc.Publish(outDir, files, doc.ValidateSite); err != nil {
					return errors.WrapErr(err, "publish examples catalog")
				}
				return nil
			}
			if outDir == "" {
				cmd.Print(md)
				return nil
			}
			return doc.Publish(outDir, map[string][]byte{"examples.md": []byte(strings.TrimSpace(md) + "\n")}, doc.ValidateMarkdownSite)
		},
	}
	cmd.Flags().StringP("output", "o", "", "directory for the catalog page (default: stdout)")
	cmd.Flags().Bool("html", false, "write the catalog as an HTML page instead of Markdown")
	return cmd
}
