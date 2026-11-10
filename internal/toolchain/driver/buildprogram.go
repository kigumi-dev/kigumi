package driver

import (
	"fmt"
	"sort"
	"strings"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/buildgraph"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/module"
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
	root, err := resolveModuleRoot(opts.Compiler, root)
	if err != nil {
		return nil, false, nil
	}
	tree, ok, err := ReadBuildFile(root)
	if err != nil || !ok {
		return nil, ok, err
	}
	// The build program sees dependencies and std only, so the module's own packages are never roots; what build.kg imports is.
	opts.RootDirs, opts.RootImports = []string{}, importPaths(tree)
	m, err := LoadModule(root, opts)
	if err != nil {
		return nil, true, err
	}
	res, err := checkBuildFileContext(m, tree)
	if err != nil {
		return nil, true, err
	}
	if res.HasErrors() {
		return nil, true, fmt.Errorf("%s", res.Render())
	}
	configure, d := configureEntity(res, tree)
	if d != nil {
		return nil, true, fmt.Errorf("%s", d.Msg)
	}
	return &BuildProgram{Module: m, Res: res, Configure: configure, Tree: tree, stdRoot: opts.StdRoot}, true, nil
}

// configureEntity finds `fn configure(b: build.Builder) -> Void!` and
// checks its shape, the way the checker checks `main`.
func configureEntity(res *sem.Result, tree *syntax.Tree) (sem.EntityID, *diag.Diagnostic) {
	at := func(sp token.Span, format string, args ...any) *diag.Diagnostic {
		return &diag.Diagnostic{Severity: diag.Error, Loc: diag.At(tree.File, sp), Msg: fmt.Sprintf(format, args...)}
	}
	pkg := res.PackageByPath("build")
	fn := res.PackageMember(pkg, "configure")
	if fn == 0 || res.Entity(fn).Kind != sem.EntFn {
		return 0, at(token.Span{}, "%s needs `fn configure(b: build.Builder) -> Void!`", BuildFileName)
	}
	sp := tree.Toks[res.Entity(fn).Tok].Span
	info := res.Fn(fn)
	if info.Set != 0 && len(res.Overloads[info.Set].Members) > 1 {
		return 0, at(sp, "%s declares configure %d times; keep the one taking build.Builder", BuildFileName, len(res.Overloads[info.Set].Members))
	}
	sig := res.Types.Node(info.Sig)
	ret, _, isResult := res.Types.IsResult(sig.Elem)
	if info.Recv != sem.RecvNone || len(info.TypeParams) != 0 || len(info.Params) != 1 || !isResult || ret != sem.TyUnit || res.TypeString(sig.Args[0]) != "Builder" {
		return 0, at(sp, "%s: configure must be `fn configure(b: build.Builder) -> Void!`", BuildFileName)
	}
	for d := range res.Tree(res.Entity(fn).File).List(res.Tree(res.Entity(fn).File).Root).Views() {
		if !d.Is(syntax.ImportDecl, syntax.FnDecl) {
			return 0, at(d.Span(), "%s holds only imports and functions; put other code in a package", BuildFileName)
		}
	}
	return fn, nil
}

// Run evaluates configure in the sandbox for a requested target and the flags
// given after `--`, and returns the recorded graph.
func (p *BuildProgram) Run(target buildconfig.Target, flags []string, outDir string) (*buildgraph.Graph, error) {
	roots, err := p.Roots(outDir)
	if err != nil {
		return nil, err
	}
	req := buildgraph.Request{Target: buildgraph.Target{Triple: target.Triple, Sys: target.Sys, Allocator: target.Allocator}, Flags: []string{}}
	for _, f := range flags {
		req.Flags = append(req.Flags, strings.TrimLeft(f, "-"))
	}
	host := p.Module.compiler.HostOrAmbient()
	if p.prog == nil {
		prog, err := buildProgram(p.Res, progOptions{verifyEach: p.Module.verifyPasses, target: p.Module.target})
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
	root, err := p.Module.compiler.Abs(p.Module.Root)
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
		dir, _, err := module.DepDir(p.Module.compiler, p.Module.local, p.Module.Selected[name], true)
		if err != nil {
			return nil, err
		}
		roots = append(roots, dir)
	}
	out, err := p.Module.compiler.Abs(outDir)
	if err != nil {
		return nil, err
	}
	return append(roots, out), nil
}
