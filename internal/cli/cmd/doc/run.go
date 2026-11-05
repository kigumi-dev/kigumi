package doc

import (
	"fmt"
	"sort"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
	docgen "kigumi/internal/tooling/doc"
)

func run(io session.IO, opts options) error {
	gen := docgen.Options{All: opts.all, Std: opts.target == "std"}
	m, err := load(io, opts.session, opts.target, gen.Std)
	if err != nil {
		return err
	}
	pages := docgen.Module(m, gen)
	if opts.html {
		if opts.outDir == "" {
			return &exit.UsageError{Err: errors.NewErrf("--html needs -o <dir>")}
		}
		title := opts.target
		if !gen.Std {
			title = m.Name
		}
		files := make(map[string][]byte)
		site, err := docgen.HTML(pages, title)
		if err != nil {
			return errors.WrapErr(err, "render doc site")
		}
		for name, text := range site {
			files[name] = []byte(text)
		}
		if err := docgen.Publish(opts.outDir, files, docgen.ValidateSite); err != nil {
			return errors.WrapErr(err, "publish doc site")
		}
		return nil
	}
	paths := make([]string, 0, len(pages))
	for p := range pages {
		paths = append(paths, p)
	}
	if opts.outDir == "" {
		sort.Strings(paths)
		for _, p := range paths {
			fmt.Fprint(io.Out, pages[p])
			fmt.Fprintln(io.Out)
		}
		return nil
	}
	files := make(map[string][]byte, len(pages)+1)
	for p, text := range pages {
		files[p+".md"] = []byte(text)
	}
	indexName := docgen.IndexFileName(paths, ".md", "README.md")
	files[indexName] = []byte(strings.TrimSpace(docgen.Index(pages)) + "\n")
	if err := docgen.Publish(opts.outDir, files, docgen.ValidateMarkdownSite); err != nil {
		return errors.WrapErr(err, "publish doc pages")
	}
	return nil
}

func load(io session.IO, s session.Session, target string, std bool) (*driver.Module, error) {
	if !std {
		return s.LoadWhole(target, false, io.Err)
	}
	stdRoot := s.StdRoot()
	m, err := driver.LoadModule(stdRoot, driver.LoadOptions{StdRoot: stdRoot, Compiler: session.CompilerOptions()})
	if err != nil {
		return nil, errors.WrapErr(err, "load std")
	}
	return m, nil
}
