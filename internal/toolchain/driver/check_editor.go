package driver

import (
	"context"
	"fmt"

	midir "kigumi/internal/compiler/mir/ir"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
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
	if err != nil || res.HasErrors() {
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
func editorDiags(m *Module) (*sem.Result, []midir.FileDiag) {
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
func editorDiagsWith(m *Module, lazy bool) (*sem.Result, []midir.FileDiag, bool) {
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
	// Lowering appends the ownership diagnostics to res.Files; snapshot the
	// semantic set first so those diagnostics are not mistaken for known ones.
	known := diagKeys(res)
	prog := lowerProgram(res, false, nil)
	if lazy && len(prog.Skipped) > 0 {
		return nil, nil, false
	}
	// Lowering errors (notably the ownership pass) land in res.Files; collect
	// them before the plan and return them even when they stop the build.
	out := append([]midir.FileDiag(nil), prog.Diags...)
	for id := range res.Files {
		for _, d := range res.Files[id].Diags {
			if !known[fmt.Sprint(id, d.Loc.Span, d.Code, d.Msg)] {
				out = append(out, midir.FileDiag{File: sem.FileID(id), Diag: d})
			}
		}
	}
	if res.HasErrors() {
		return res, out, true
	}
	lowered := len(out)
	prog.NativeBoxes = nativeBoxSites(m.target)
	if mirplan.Apply(prog, planPasses(false)...) != nil {
		return res, out, true
	}
	for _, d := range prog.Diags {
		if !containsFileDiag(out[:lowered], d) {
			out = append(out, d)
		}
	}
	for _, c := range claimDiags(prog) {
		out = append(out, midir.FileDiag(c))
	}
	for id := range res.Files {
		for _, d := range res.Files[id].Diags {
			if !known[fmt.Sprint(id, d.Loc.Span, d.Code, d.Msg)] {
				out = append(out, midir.FileDiag{File: sem.FileID(id), Diag: d})
			}
		}
	}
	return res, out, true
}

func containsFileDiag(ds []midir.FileDiag, d midir.FileDiag) bool {
	for _, x := range ds {
		if x.File == d.File && x.Diag.Loc.Span == d.Diag.Loc.Span && x.Diag.Code == d.Diag.Code && x.Diag.Msg == d.Diag.Msg {
			return true
		}
	}
	return false
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
