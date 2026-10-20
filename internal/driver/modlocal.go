package driver

import (
	"fmt"
	"path/filepath"
	"strings"

	"kigumi/internal/diag"
	"kigumi/internal/token"
)

// LocalName holds the developer's Replace records. It is looked up from
// the module root upwards and only the nearest one counts, so a published
// module can never redirect its users.
const LocalName = "mod.local.kg"

var localFields = map[string][]string{"Replace": {"name", "path"}}

// Local is a parsed mod.local.kg: replaced module names mapped to absolute
// directories. Provenance carries the same keys mapped to "file:line" of
// the Replace record, for release-mode diagnostics.
type Local struct {
	File       string
	Replaces   map[string]string
	Provenance map[string]string
}

// LocalReplaceDir reports the directory a Replace record sent name to, so
// callers can tell a developer's checkout apart from the module cache.
func (m *Module) LocalReplaceDir(name string) (string, bool) {
	p, ok := m.local.Replaces[name]
	return p, ok
}

// ReadLocal finds the nearest mod.local.kg at or above dir.
func ReadLocal(o CompilerOptions, dir string) (Local, error) {
	// A relative dir would stop the walk at "." before reaching any parent.
	if abs, err := o.Abs(dir); err == nil {
		dir = abs
	}
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		path := filepath.Join(d, LocalName)
		src, ok, err := readFileMaybe(path)
		if err != nil {
			return Local{}, err
		}
		if ok {
			f := token.NewFile(LocalName, src)
			lc, ds := ParseLocal(f, d)
			if len(ds) > 0 {
				return lc, fmt.Errorf("%s: %s", path, diag.RenderAll(f, ds[:1]))
			}
			lc.File = path
			for name, loc := range lc.Provenance {
				lc.Provenance[name] = path + strings.TrimPrefix(loc, f.Name)
			}
			return lc, nil
		}
		if filepath.Dir(d) == d {
			return Local{Replaces: map[string]string{}}, nil
		}
	}
}

// ParseLocal decodes Replace records; paths are resolved against dir.
func ParseLocal(f *token.File, dir string) (Local, []diag.Diagnostic) {
	lc := Local{Replaces: map[string]string{}, Provenance: map[string]string{}}
	recs, ds := parseRecords(f, localFields)
	for _, r := range recs {
		p := r.Fields["path"]
		if p == "" {
			ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, "Replace needs a path"))
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		lc.Replaces[r.Fields["name"]] = p
		lc.Provenance[r.Fields["name"]] = fmt.Sprintf("%s:%d", f.Name, f.Line(r.Span.Start))
	}
	return lc, ds
}
