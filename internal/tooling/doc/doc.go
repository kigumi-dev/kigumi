// Package doc renders the declarations of a module as Markdown: one
// document per package, each declaration shown as written with its `///`
// comment, methods grouped under their receiver type.
package doc

import (
	"sort"
	"strconv"
	"strings"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/toolchain/driver"
)

type Options struct {
	// All includes declarations without `pub`.
	All bool
	// Std selects the std packages instead of the module's own.
	Std bool
}

// Module renders every selected package, keyed by package path.
func Module(m *driver.Module, opts Options) map[string]string {
	out := map[string]string{}
	for _, path := range m.Order {
		p := m.Packages[path]
		if p.Std != opts.Std {
			continue
		}
		if text := Package(p, opts); text != "" {
			out[path] = text
		}
	}
	return out
}

// IndexFileName picks the site-relative filename for the generated package-list page, renaming it if a package already uses def.
func IndexFileName(paths []string, ext, def string) string {
	used := make(map[string]bool, len(paths))
	for _, p := range paths {
		used[p+ext] = true
	}
	if !used[def] {
		return def
	}
	base := strings.TrimSuffix(def, ext)
	for i := 2; ; i++ {
		name := base + "-" + strconv.Itoa(i) + ext
		if !used[name] {
			return name
		}
	}
}

// Index lists the rendered packages.
func Index(pages map[string]string) string {
	paths := make([]string, 0, len(pages))
	for p := range pages {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var sb strings.Builder
	sb.WriteString("# Packages\n\n")
	for _, p := range paths {
		sb.WriteString("- [" + p + "](" + p + ".md)\n")
	}
	return sb.String()
}

// Package renders one package; "" when it has nothing to show.
func Package(p *driver.Package, opts Options) string {
	var types, ifaces, fns, consts, statics, reexports []entry
	methods := map[string][]entry{}
	var owners []string
	for _, t := range p.Files {
		if strings.HasSuffix(t.File.Name, "_test.kg") {
			continue
		}
		for d := range t.List(t.Root).IDs() {
			for _, e := range declEntries(t, d, opts.All) {
				switch {
				case e.kind == syntax.FnDecl && e.recv != "":
					if _, seen := methods[e.recv]; !seen {
						owners = append(owners, e.recv)
					}
					methods[e.recv] = append(methods[e.recv], e)
				case e.kind == syntax.FnDecl:
					fns = append(fns, e)
				case e.kind == syntax.TypeDecl:
					types = append(types, e)
				case e.kind == syntax.InterfaceDecl:
					ifaces = append(ifaces, e)
				case e.kind == syntax.ConstDecl:
					consts = append(consts, e)
				case e.kind == syntax.StaticDecl:
					statics = append(statics, e)
				case e.kind == syntax.ImportDecl:
					reexports = append(reexports, e)
				}
			}
		}
	}
	if len(types)+len(ifaces)+len(fns)+len(consts)+len(statics)+len(reexports)+len(methods) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# " + p.Path + "\n")
	section(&sb, "Types", types, func(e entry) {
		for _, m := range methods[e.name] {
			write(&sb, "#### ", m)
		}
		delete(methods, e.name)
	})
	section(&sb, "Interfaces", ifaces, nil)
	for _, owner := range owners {
		if ms, ok := methods[owner]; ok {
			sb.WriteString("\n## Methods of " + owner + "\n")
			for _, m := range ms {
				write(&sb, "### ", m)
			}
		}
	}
	section(&sb, "Functions", fns, nil)
	section(&sb, "Constants", consts, nil)
	section(&sb, "Statics", statics, nil)
	section(&sb, "Re-exports", reexports, nil)
	return sb.String()
}

func section(sb *strings.Builder, title string, es []entry, after func(entry)) {
	if len(es) == 0 {
		return
	}
	sb.WriteString("\n## " + title + "\n")
	for _, e := range es {
		write(sb, "### ", e)
		if after != nil {
			after(e)
		}
	}
}

func write(sb *strings.Builder, prefix string, e entry) {
	name := e.name
	if e.recv != "" {
		name = e.recv + "." + name
	}
	sb.WriteString("\n" + prefix + name + "\n\n```kigumi\n" + e.head + "\n```\n")
	if e.doc != "" {
		sb.WriteString("\n" + e.doc + "\n")
	}
}
