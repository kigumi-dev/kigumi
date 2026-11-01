package doc

import (
	"fmt"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	docgen "kigumi/internal/tooling/doc"
)

func runExamples(io session.IO, opts examplesOptions) error {
	md, err := docgen.ExamplesCatalogMarkdown(opts.root)
	if err != nil {
		return errors.WrapErr(err, "load examples catalog")
	}
	if opts.html {
		if opts.outDir == "" {
			return &exit.UsageError{Err: errors.NewErrf("--html needs -o <dir>")}
		}
		site := docgen.HTML(map[string]string{"examples": md}, "Examples")
		files := make(map[string][]byte, len(site))
		for name, text := range site {
			files[name] = []byte(text)
		}
		if err := docgen.Publish(opts.outDir, files, docgen.ValidateSite); err != nil {
			return errors.WrapErr(err, "publish examples catalog")
		}
		return nil
	}
	if opts.outDir == "" {
		fmt.Fprint(io.Msg, md)
		return nil
	}
	return docgen.Publish(opts.outDir, map[string][]byte{"examples.md": []byte(strings.TrimSpace(md) + "\n")}, docgen.ValidateMarkdownSite)
}
