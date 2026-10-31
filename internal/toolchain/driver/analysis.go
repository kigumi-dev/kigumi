package driver

import (
	"kigumi/internal/backend/llgen"
	"kigumi/internal/compiler/mir"
	resbound "kigumi/internal/compiler/mir/resource"
	"kigumi/internal/compiler/sem"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/contract/schema"
)

// Analysis is the checked view of a module, published complete and never written afterward.
type Analysis struct {
	Res *sem.Result
	// Prog is nil when the checker reported errors.
	Prog       *mir.Program
	Provenance []schema.GeneratedArtifact
}

// Stats counts the phase runs of one module, so a test can pin that one
// build invocation checks, lowers and emits exactly once.
type Stats struct {
	Check, MIR, Emit int
}

// Analyze checks a loaded module and lowers it to MIR once. The error is
// a compiler fault: the MIR the builder or a pass produced broke the MIR
// contract (mir.Program.Verify), never a diagnostic of the program.
func Analyze(m *Module) (*Analysis, error) {
	return AnalyzeWith(m, nil)
}

// AnalyzeWith is Analyze with an explicit sem.DeriveGenerator (nil keeps
// Analyze's default); a caller overrides it to compare generators or check a module that can't derive.
func AnalyzeWith(m *Module, gen sem.DeriveGenerator) (*Analysis, error) {
	if m.lazyStd && !m.lowerAll {
		if a, ok, err := m.analyzeLazyStd(gen); ok || err != nil {
			return a, err
		}
	}
	mod, err := m.semModule()
	if err != nil {
		return nil, err
	}
	return m.analyze(mod, gen)
}

func (m *Module) analyze(mod *sem.Module, gen sem.DeriveGenerator) (*Analysis, error) {
	a, err := m.analyzeCore(mod, gen, nil)
	if a != nil {
		m.recordPackages(a.Res, a.Prog != nil)
	}
	return a, err
}

// analyzeCore is analyze without recording artifacts; keep, when set, limits which own packages are lowered up front.
func (m *Module) analyzeCore(mod *sem.Module, gen sem.DeriveGenerator, keep func(path string) bool) (*Analysis, error) {
	m.stats.Check++
	if gen == nil {
		gen = schema.GenerateDerive
	}
	res := sem.CheckWith(mod, gen)
	if m.HasParseErrors() {
		res.FailParse()
	}
	a := &Analysis{Res: res, Provenance: schema.BuildProvenance(res, nil)}
	m.lastAnalysis = a
	if res.HasErrors() {
		return a, nil
	}
	m.stats.MIR++
	var err error
	if a.Prog, err = buildProgram(res, progOptions{strict: m.target.StrictStack, budgetKiB: m.target.StackBudgetKiB, verifyEach: m.verifyPasses, lowerAll: m.lowerAll, rootKeep: keep, target: m.target}); err != nil {
		return nil, err
	}
	// Several cmd/ scripts each have an implicit main; the module's entry
	// names the one a program starts from.
	if m.Entry != "" {
		for _, ent := range res.ImplicitMains() {
			if res.Tree(res.Entity(ent).File).File.Name == m.Entry {
				a.Prog.Entry = a.Prog.ByEnt[ent]
			}
		}
	}
	return a, nil
}

// progOptions shape buildProgram: strict and budgetKiB gate the resource summary (the build program
// itself always passes hosted-growable), and lowerAll lowers bodies nothing reaches as well.
type progOptions struct {
	strict     bool
	budgetKiB  int
	verifyEach bool
	lowerAll   bool
	// rootKeep limits the own packages lowered up front.
	rootKeep func(path string) bool
	// target is what the program is built for, which the native lowering's boxes depend on.
	target buildconfig.Target
}

// buildProgram returns a program with diagnostics unverified.
func buildProgram(res *sem.Result, o progOptions) (*mir.Program, error) {
	prog := lowerProgram(res, o.lowerAll, o.rootKeep)
	if res.HasErrors() || len(prog.Skipped) > 0 {
		return prog, nil
	}
	prog.VerifyEach = o.verifyEach
	prog.NativeBoxes = nativeBoxSites(o.target)
	if err := prog.Apply(planPasses(true)...); err != nil {
		return prog, err
	}
	prog.Resources = resbound.Compute(prog.R, prog.Funcs, prog.ByEnt)
	resbound.SettleAsync(prog.R, prog.Funcs, prog.Resources)
	prog.Diags = append(prog.Diags, resbound.CheckBudget(prog.R, prog.ByEnt, prog.Resources, resbound.Options{Strict: o.strict, BudgetKiB: o.budgetKiB})...)
	for _, d := range prog.Diags {
		res.Files[d.File].Diags = append(res.Files[d.File].Diags, d.Diag)
	}
	for _, c := range append(mir.CheckAllocClaims(prog), mir.CheckInterruptLowering(prog)...) {
		res.Files[c.File].Diags = append(res.Files[c.File].Diags, c.Diag)
	}
	return prog, nil
}

func nativeBoxSites(t buildconfig.Target) func(*mir.Program) mir.NativeBoxSites {
	return func(p *mir.Program) mir.NativeBoxSites {
		opts := emitOptionsFor(t)
		opts.Reachable = mir.Reachable(p)
		return llgen.ScalarBoxSites(p, opts)
	}
}

func lowerProgram(res *sem.Result, all bool, keep func(path string) bool) *mir.Program {
	var prog *mir.Program
	switch {
	case all:
		prog = mir.BuildAll(res)
	case keep != nil:
		prog = mir.BuildFor(res, keep)
	default:
		prog = mir.Build(res)
	}
	for _, f := range prog.Funcs {
		res.Files[f.File].Diags = append(res.Files[f.File].Diags, f.Diags...)
	}
	for _, cf := range prog.Comptime {
		res.Files[cf.Func.File].Diags = append(res.Files[cf.Func.File].Diags, cf.Func.Diags...)
	}
	return prog
}

// fold-comptime runs last so a comptime block runs the planned MIR.
func planPasses(comptime bool) []mir.Transform {
	out := []mir.Transform{
		{Name: "plan-rc", Run: mir.PlanRC},
		{Name: "specialize", Run: mir.Specialize},
		{Name: "elide-rc", Run: mir.ElideRC},
		{Name: "plan-direct", Run: mir.PlanDirect},
	}
	if comptime {
		out = append(out, mir.Transform{Name: "fold-comptime", Run: foldComptime})
	}
	return out
}

// Stats reports how often each phase ran for this module.
func (m *Module) Stats() Stats { return m.stats }

// LastAnalysis is the most recent Analyze result for this module, nil
// before any call; a resource report reads it instead of re-analyzing.
func (m *Module) LastAnalysis() *Analysis { return m.lastAnalysis }
