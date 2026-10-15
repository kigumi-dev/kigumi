package driver

import (
	"fmt"
	"kigumi/internal/buildgraph"
	"path/filepath"
	"sort"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/vm"
)

// BuildProgram is a checked build.kg: the module it configures, the checker
// result over std and the dependencies, and its configure entity.
type BuildProgram struct {
	Module    *Module
	Res       *sem.Result
	Configure sem.EntityID
	Tree      *syntax.Tree
	stdRoot   string
	prog      *mir.Program
}

// LoadBuildProgram loads root's module and checks its build.kg as a package named
// build; ok is false when the module has no build program.
func LoadBuildProgram(root string, opts LoadOptions) (*BuildProgram, bool, error) {
	tree, ok, err := ReadBuildFile(root)
	if err != nil || !ok {
		return nil, ok, err
	}
	m, err := LoadModule(root, opts)
	if err != nil {
		return nil, true, err
	}
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
	for _, path := range importPaths(tree) {
		if p, ok := m.Packages[path]; ok && !p.Std && p.Module == "" {
			return nil, true, fmt.Errorf("%s cannot import %s, a package of the module it builds; call a helper a dependency exports or select a Framework", BuildFileName, path)
		}
	}
	mod.Packages = append(mod.Packages, &sem.Package{Path: "build", Files: []*syntax.Tree{tree}})
	a, err := (&Module{verifyPasses: opts.VerifyPasses}).analyze(mod, nil)
	if err != nil {
		return nil, true, err
	}
	res := a.Res
	if res.HasErrors() {
		return nil, true, fmt.Errorf("%s", res.Render())
	}
	configure, err := configureEntity(res)
	if err != nil {
		return nil, true, err
	}
	return &BuildProgram{Module: m, Res: res, Configure: configure, Tree: tree, stdRoot: opts.StdRoot}, true, nil
}

// configureEntity finds `fn configure(b: build.Builder) -> Unit!` and
// checks its shape, the way the checker checks `main`.
func configureEntity(res *sem.Result) (sem.EntityID, error) {
	pkg := res.PackageByPath("build")
	fn := res.PackageMember(pkg, "configure")
	if fn == 0 || res.Entity(fn).Kind != sem.EntFn {
		return 0, fmt.Errorf("%s needs `fn configure(b: build.Builder) -> Unit!`", BuildFileName)
	}
	info := res.Fn(fn)
	if info.Set != 0 && len(res.Overloads[info.Set].Members) > 1 {
		return 0, fmt.Errorf("%s declares configure %d times; keep the one taking build.Builder", BuildFileName, len(res.Overloads[info.Set].Members))
	}
	sig := res.Types.Node(info.Sig)
	ret, _, isResult := res.Types.IsResult(sig.Elem)
	if info.Recv != sem.RecvNone || len(info.TypeParams) != 0 || len(info.Params) != 1 || !isResult || ret != sem.TyUnit || res.TypeString(sig.Args[0]) != "Builder" {
		return 0, fmt.Errorf("%s: configure must be `fn configure(b: build.Builder) -> Unit!`", BuildFileName)
	}
	for d := range res.Tree(res.Entity(fn).File).List(res.Tree(res.Entity(fn).File).Root).Views() {
		if !d.Is(syntax.ImportDecl, syntax.FnDecl) {
			return 0, fmt.Errorf("%s holds only imports and functions; put other code in a package", BuildFileName)
		}
	}
	return fn, nil
}

// Run evaluates configure in the sandbox for a requested target and the flags
// given after `--`, and returns the recorded graph.
func (p *BuildProgram) Run(target Target, flags []string, outDir string) (*buildgraph.Graph, error) {
	roots, err := p.Roots(outDir)
	if err != nil {
		return nil, err
	}
	req := buildgraph.Request{Target: buildgraph.Target{Triple: target.Triple, Sys: target.Sys, Allocator: target.Allocator}, Flags: []string{}}
	for _, f := range flags {
		req.Flags = append(req.Flags, strings.TrimLeft(f, "-"))
	}
	host := HostTarget()
	if p.prog == nil {
		prog, err := buildProgram(p.Res, progOptions{verifyEach: p.Module.verifyPasses})
		if err != nil {
			return nil, err
		}
		if p.Res.HasErrors() {
			return nil, fmt.Errorf("%s", strings.TrimSpace(p.Res.Render()))
		}
		p.prog = prog
	}
	return vm.EvalBuild(p.prog, p.Configure, req, buildgraph.Roots{Roots: roots, Host: buildgraph.Target{Sys: host.Sys, Allocator: target.Allocator}})
}

// DescriptorRoots lists where a Framework descriptor may come from, never the
// output directory, which anyone may write to.
func (p *BuildProgram) DescriptorRoots() ([]string, error) {
	roots, err := p.Roots(".")
	if err != nil {
		return nil, err
	}
	return roots[:len(roots)-1], nil
}

// Roots lists the directories a build program may name files in: the module root,
// then every selected dependency, then the output directory.
func (p *BuildProgram) Roots(outDir string) ([]string, error) {
	root, err := filepath.Abs(p.Module.Root)
	if err != nil {
		return nil, err
	}
	roots := []string{root}
	names := make([]string, 0, len(p.Module.Selected))
	for name := range p.Module.Selected {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		dir, _, err := DepDir(p.Module.local, p.Module.Selected[name], true)
		if err != nil {
			return nil, err
		}
		roots = append(roots, dir)
	}
	out, err := filepath.Abs(outDir)
	if err != nil {
		return nil, err
	}
	return append(roots, out), nil
}
