package driver

import (
	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
)

func CheckForEditor(m *Module) (*sem.Result, error) {
	res, err := CheckOnly(m)
	if err != nil || res.HasErrors() || !res.DefersAllocClaims() {
		return res, err
	}
	built, claims := editorClaims(m)
	for _, c := range claims {
		if f := res.File(built.Tree(c.File)); f != nil {
			f.Diags = append(f.Diags, c.Diag)
		}
	}
	return res, nil
}

// A second check keeps the entities specialisation mints out of the result.
func editorClaims(m *Module) (*sem.Result, []mir.AllocClaim) {
	mod, err := m.semModule()
	if err != nil {
		return nil, nil
	}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if res.HasErrors() {
		return res, nil
	}
	if prog := lowerProgram(res); !res.HasErrors() && prog.Apply(planPasses(false)...) == nil {
		return res, mir.CheckAllocClaims(prog)
	}
	return res, nil
}
