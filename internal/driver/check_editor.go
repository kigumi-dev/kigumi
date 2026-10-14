package driver

import (
	"fmt"

	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
)

func CheckForEditor(m *Module) (*sem.Result, error) {
	res, err := CheckOnly(m)
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
	mod, err := m.semModule()
	if err != nil {
		return nil, nil
	}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if res.HasErrors() {
		return res, nil
	}
	prog := lowerProgram(res)
	if res.HasErrors() {
		return res, nil
	}
	known := diagKeys(res)
	if prog.Apply(planPasses(false)...) != nil {
		return res, nil
	}
	out := append([]mir.FileDiag(nil), prog.Diags...)
	for _, c := range mir.CheckAllocClaims(prog) {
		out = append(out, mir.FileDiag(c))
	}
	for id := range res.Files {
		for _, d := range res.Files[id].Diags {
			if !known[fmt.Sprint(id, d.Loc.Span, d.Code, d.Msg)] {
				out = append(out, mir.FileDiag{File: sem.FileID(id), Diag: d})
			}
		}
	}
	return res, out
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
