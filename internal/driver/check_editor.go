package driver

import (
	"context"
	"fmt"

	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
)

func CheckForEditor(m *Module) (*sem.Result, error) {
	return CheckForEditorContext(context.Background(), m)
}

// CheckForEditorContext is CheckForEditor that stops once ctx is done and returns its error.
func CheckForEditorContext(ctx context.Context, m *Module) (*sem.Result, error) {
	m.cancel = func() bool { return ctx.Err() != nil }
	defer func() { m.cancel = nil }()
	res, err := CheckOnly(m)
	if err == nil && res.Cancelled() {
		return nil, ctx.Err()
	}
	if err != nil || res.HasErrors() || !res.DefersAllocClaims() && !res.DeclaresNoalloc() && !res.SpecializesCalls() && !m.target.RestrictsAllocation() {
		return res, err
	}
	built, found := editorDiags(m)
	for _, c := range found {
		if f := res.File(built.Tree(c.File)); f != nil {
			f.Diags = append(f.Diags, c.Diag)
		}
	}
	return res, nil
}

// A second check keeps the entities specialisation mints out of the result.
func editorDiags(m *Module) (*sem.Result, []mir.FileDiag) {
	if m.lazyStd {
		if res, out, ok := editorDiagsWith(m, true); ok {
			m.unitStats.Lazy++
			return res, out
		}
		m.unitStats.LazyRetries++
	}
	res, out, _ := editorDiagsWith(m, false)
	return res, out
}

// editorDiagsWith is editorDiags; with lazy it skips the std bodies the module never names, and ok is false when one was needed.
func editorDiagsWith(m *Module, lazy bool) (*sem.Result, []mir.FileDiag, bool) {
	mod, err := m.semModule()
	if err != nil {
		return nil, nil, true
	}
	if lazy {
		mod.LazyBodies = func(path string) bool { p := m.Packages[path]; return p != nil && p.Std }
	}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if res.Cancelled() {
		return res, nil, true
	}
	if lazy && res.LazyMissed() {
		return nil, nil, false
	}
	if res.HasErrors() {
		return res, nil, true
	}
	prog := lowerProgram(res, false, nil)
	if lazy && len(prog.Skipped) > 0 {
		return nil, nil, false
	}
	if res.HasErrors() {
		return res, nil, true
	}
	known := diagKeys(res)
	if prog.Apply(planPasses(false)...) != nil {
		return res, nil, true
	}
	out := append([]mir.FileDiag(nil), prog.Diags...)
	for _, c := range append(mir.CheckAllocClaims(prog), mir.CheckInterruptLowering(prog)...) {
		out = append(out, mir.FileDiag(c))
	}
	for id := range res.Files {
		for _, d := range res.Files[id].Diags {
			if !known[fmt.Sprint(id, d.Loc.Span, d.Code, d.Msg)] {
				out = append(out, mir.FileDiag{File: sem.FileID(id), Diag: d})
			}
		}
	}
	return res, out, true
}

func diagKeys(res *sem.Result) map[string]bool {
	out := map[string]bool{}
	for id := range res.Files {
		for _, d := range res.Files[id].Diags {
			out[fmt.Sprint(id, d.Loc.Span, d.Code, d.Msg)] = true
		}
	}
	return out
}
