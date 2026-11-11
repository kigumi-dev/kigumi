package catalog

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// SPIMarker starts a doc line that marks the declaration as internal SPI.
// The rest of the line is the purpose shown in the catalog.
const SPIMarker = "Internal SPI:"

// facts is what the sources say about one package; the policy file cannot
// write these fields.
type facts struct {
	path         string
	declarations Declarations
	spi          []SPI
	imports      []string
	declared     map[string]bool
}

// scan parses every .kg under the std root. Test files are skipped: they are
// not part of the published package.
func scan(stdRoot string) (map[string]*facts, error) {
	out := map[string]*facts{}
	err := filepath.WalkDir(stdRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != stdRoot && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".kg") || strings.HasSuffix(name, "_test.kg") {
			return nil
		}
		rel, err := filepath.Rel(stdRoot, filepath.Dir(p))
		if err != nil {
			return err
		}
		pkg := path.Join("std", filepath.ToSlash(rel))
		f := out[pkg]
		if f == nil {
			f = &facts{path: pkg, declared: map[string]bool{}}
			out[pkg] = f
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		t := syntax.Parse(token.NewFile(path.Join(pkg, name), src))
		if err := f.collect(t); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.WrapErr(err, "scan std sources")
	}
	return out, nil
}

// collect adds one parsed file's public declarations, doc-marked SPI and
// std imports.
func (f *facts) collect(t *syntax.Tree) error {
	for d := range t.List(t.Root).IDs() {
		if t.Kind(d) == syntax.ImportDecl {
			imp := t.View(d).AsImportDecl().Path().Join("/")
			if strings.HasPrefix(imp, "std/") {
				f.imports = append(f.imports, imp)
			}
			continue
		}
		decl := t.View(d).AsDecl()
		name := decl.Name()
		if !name.Valid() {
			continue
		}
		purpose, marked := spiDoc(syntax.DocText(t, d))
		if !decl.Vis().Valid() {
			if marked {
				return errors.NewErrf("%s: %s carries an %s marker but is not pub", f.path, name.Text(), SPIMarker)
			}
			continue
		}
		full := name.Text()
		if t.Kind(d) == syntax.FnDecl {
			if recv := t.View(d).AsFnDecl().Recv(); recv.Valid() {
				if owner := recv.Path().Join("."); owner != "" {
					full = owner + "." + full
				}
			}
			f.declarations.Functions++
		} else {
			switch t.Kind(d) {
			case syntax.TypeDecl:
				f.declarations.Types++
			case syntax.InterfaceDecl:
				f.declarations.Interfaces++
			case syntax.ConstDecl:
				f.declarations.Constants++
			case syntax.StaticDecl:
				f.declarations.Statics++
			}
		}
		f.declared[full] = true
		if marked {
			f.spi = append(f.spi, SPI{Name: full, Purpose: purpose})
		}
	}
	return nil
}

// spiDoc returns the purpose after the last SPIMarker line.
func spiDoc(doc string) (string, bool) {
	purpose, ok := "", false
	for _, line := range strings.Split(doc, "\n") {
		if rest, found := strings.CutPrefix(strings.TrimSpace(line), SPIMarker); found {
			purpose, ok = strings.TrimSpace(rest), true
		}
	}
	return purpose, ok
}
