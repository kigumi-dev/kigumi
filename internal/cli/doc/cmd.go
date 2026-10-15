// Package doc implements `kigumi doc`: render the declarations of a module
// or of std as Markdown.
package doc

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/cliutil"
	"kigumi/internal/doc"
	"kigumi/internal/driver"
	"kigumi/internal/errors"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc [-o <dir>] [--all] [--html] <module-root|std>",
		Short: "Render the documentation of a module (or of std) as Markdown, or as a static HTML site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			outDir, _ := cmd.Flags().GetString("output")
			opts := doc.Options{All: all, Std: args[0] == "std"}
			var m *driver.Module
			var err error
			if opts.Std {
				stdRoot := shared.StdRoot(cmd)
				m, err = driver.LoadModule(stdRoot, driver.LoadOptions{StdRoot: stdRoot})
				if err != nil {
					err = errors.WrapErr(err, "load std")
				}
			} else {
				m, err = shared.LoadWhole(cmd, args[0], false, cmd.ErrOrStderr())
			}
			if err != nil {
				return err
			}
			pages := doc.Module(m, opts)
			if asHTML, _ := cmd.Flags().GetBool("html"); asHTML {
				if outDir == "" {
					return &cliutil.UsageError{Err: errors.NewErrf("--html needs -o <dir>")}
				}
				title := args[0]
				if !opts.Std {
					title = m.Name
				}
				files := make(map[string][]byte)
				for name, text := range doc.HTML(pages, title) {
					files[name] = []byte(text)
				}
				if err := doc.Publish(outDir, files, doc.ValidateSite); err != nil {
					return errors.WrapErr(err, "publish doc site")
				}
				return nil
			}
			if outDir == "" {
				paths := make([]string, 0, len(pages))
				for p := range pages {
					paths = append(paths, p)
				}
				sort.Strings(paths)
				for _, p := range paths {
					cmd.Print(pages[p])
					cmd.Println()
				}
				return nil
			}
			paths := make([]string, 0, len(pages))
			for p := range pages {
				paths = append(paths, p)
			}
			files := make(map[string][]byte, len(pages)+1)
			for p, text := range pages {
				files[p+".md"] = []byte(text)
			}
			indexName := doc.IndexFileName(paths, ".md", "README.md")
			files[indexName] = []byte(strings.TrimSpace(doc.Index(pages)) + "\n")
			if err := doc.Publish(outDir, files, doc.ValidateMarkdownSite); err != nil {
				return errors.WrapErr(err, "publish doc pages")
			}
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "directory for one .md per package plus README.md (default: stdout)")
	cmd.Flags().Bool("all", false, "include declarations without pub")
	cmd.Flags().Bool("html", false, "write a static HTML site (index.html and one page per package) instead of Markdown")
	cmd.AddCommand(ExamplesCmd())
	return cmd
}
