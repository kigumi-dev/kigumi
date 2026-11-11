package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// BuildFile is a root's build.kg parsed and checked the way `kigumi build`
// checks it before evaluating configure. Checking never runs the program:
// nothing is evaluated, no file is written and no tool runs. Res is nil when
// build.kg did not parse.
type BuildFile struct {
	Tree *syntax.Tree
	Res  *sem.Result
	// Err is a failure that belongs to build.kg as a whole but produces no
	// checker diagnostic: an unresolvable dependency, an import of the
	// module being built, or a configure without the fixed shape.
	Err *diag.Diagnostic
}

// HasErrors reports whether build.kg stands in the way of `kigumi build`.
func (b *BuildFile) HasErrors() bool {
	if b == nil || b.Tree == nil {
		return false
	}
	return b.Err != nil || b.Tree.HasErrors() || b.Res != nil && b.Res.HasErrors()
}

// Diagnostics returns the diagnostics bound to build.kg itself. A diagnostic
// the checker reported in std or a dependency is left out, so an editor never
// attributes a dependency's error to the build program's file.
func (b *BuildFile) Diagnostics() []diag.Diagnostic {
	if b == nil || b.Tree == nil {
		return nil
	}
	out := slices.Clone(b.Tree.Diags)
	if b.Res != nil {
		out = append(out, b.Res.Diagnostics(b.Tree)...)
	}
	if b.Err != nil {
		out = append(out, *b.Err)
	}
	return out
}

// Render formats build.kg's diagnostics, "" when it is clean.
func (b *BuildFile) Render() string {
	switch {
	case b == nil || b.Tree == nil:
		return ""
	case b.Res != nil:
		return diag.RenderAll(b.Res, b.Diagnostics())
	default:
		return diag.RenderAll(b.Tree.File, b.Diagnostics())
	}
}

// CheckBuildFile loads root's module and checks its build.kg without running
// configure; ok is false when root has no build.kg. A load failure is reported
// as a diagnostic in the returned BuildFile rather than as the error, so a
// caller that checks the module too can report both.
func CheckBuildFile(root string, opts LoadOptions) (*BuildFile, bool, error) {
	root, err := resolveModuleRoot(opts.Compiler, root)
	if err != nil {
		return nil, false, nil
	}
	tree, ok, err := parseBuildFile(root, opts.Overlay)
	if err != nil || !ok {
		return nil, ok, err
	}
	return checkBuildFile(root, tree, opts), true, nil
}

// CheckBuildFileIn checks root/build.kg, read through overlay, against m, a
// module whose packages already include std and every dependency build.kg can
// import (the editor loads with AllPackages). It loads nothing and never runs
// configure; ok is false when root has no build.kg.
func CheckBuildFileIn(m *Module, overlay map[string][]byte) (*BuildFile, bool, error) {
	tree, ok, err := parseBuildFile(m.Root, overlay)
	if err != nil || !ok {
		return nil, ok, err
	}
	return checkBuildFileOn(m, tree), true, nil
}

// checkBuildFile finishes CheckBuildFile: a parse error needs no module, and a
// load failure becomes a file-level diagnostic at the top of build.kg.
func checkBuildFile(root string, tree *syntax.Tree, opts LoadOptions) *BuildFile {
	if tree.HasErrors() {
		return &BuildFile{Tree: tree}
	}
	// The build program sees dependencies and std only, so the module's own packages are never roots; what build.kg imports is.
	opts.RootDirs, opts.RootImports = []string{}, importPaths(tree)
	m, err := LoadModule(root, opts)
	if err != nil {
		return buildFileFailure(tree, err)
	}
	return checkBuildFileOn(m, tree)
}

func checkBuildFileOn(m *Module, tree *syntax.Tree) *BuildFile {
	if tree.HasErrors() {
		return &BuildFile{Tree: tree}
	}
	res, err := checkBuildFileContext(m, tree)
	if err != nil {
		return buildFileFailure(tree, err)
	}
	b := &BuildFile{Tree: tree, Res: res}
	if !res.HasErrors() {
		if _, d := configureEntity(res, tree); d != nil {
			b.Err = d
		}
	}
	return b
}

// checkBuildFileContext checks tree as the package named build over m's std
// and dependency packages. It never includes the module build.kg configures.
func checkBuildFileContext(m *Module, tree *syntax.Tree) (*sem.Result, error) {
	mod := &sem.Module{}
	for _, path := range m.Order {
		p := m.Packages[path]
		if !p.Std && p.Module == "" {
			continue
		}
		sp := &sem.Package{Path: p.Path, Files: p.Files, Std: p.Std, Module: p.Module}
		if p.Std {
			sp.Module = "std"
		}
		mod.Packages = append(mod.Packages, sp)
	}
	if m.index != nil {
		for _, path := range importPaths(tree) {
			if ip, ok := m.index.pkgs[path]; ok && !ip.std && ip.module == "" {
				return nil, fmt.Errorf("%s cannot import %s, a package of the module it builds; call a helper a dependency exports or select a Framework", BuildFileName, path)
			}
		}
	}
	mod.Packages = append(mod.Packages, &sem.Package{Path: "build", Files: []*syntax.Tree{tree}})
	mod.BuildProgram = "build"
	a, err := (&Module{verifyPasses: m.verifyPasses}).analyze(mod, nil)
	if err != nil {
		return nil, err
	}
	a.Snapshot()
	return a.Res, nil
}

// buildFileFailure locates a build.kg-wide failure at the file's first byte,
// the only place an error without a source span can point.
func buildFileFailure(tree *syntax.Tree, err error) *BuildFile {
	return &BuildFile{Tree: tree, Err: &diag.Diagnostic{Severity: diag.Error, Loc: diag.At(tree.File, token.Span{Start: 0, End: 0}), Msg: err.Error()}}
}

// parseBuildFile parses root/build.kg, reading overlay first so an editor's
// unsaved build.kg is what gets checked. ok is false when there is no file.
func parseBuildFile(root string, overlay map[string][]byte) (*syntax.Tree, bool, error) {
	path := filepath.Join(root, BuildFileName)
	src, ok := overlay[path]
	if !ok {
		var err error
		src, err = os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, false, nil
			}
			return nil, false, err
		}
	}
	return syntax.Parse(token.NewFile(BuildFileName, src)), true, nil
}
