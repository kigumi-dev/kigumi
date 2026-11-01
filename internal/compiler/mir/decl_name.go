package mir

import (
	"fmt"

	"kigumi/internal/compiler/sem"
)

func (p *Program) FuncName(ent sem.EntityID) string {
	e := p.R.Entity(ent)
	switch e.Kind {
	case sem.EntImplicitMain:
		return "main"
	case sem.EntTest:
		return "test." + e.Name
	case sem.EntClosure:
		return fmt.Sprintf("%s.closure%d", p.FuncName(e.Parent), p.R.ClosureOrdinal(ent))
	}
	if id := p.R.DeclID(ent); !id.IsZero() {
		return id.String()
	}
	return p.R.Packages[e.Pkg].Path + "." + e.Name
}

// DeclName spells the declaration an instruction names; a closure uses its function name.
func (p *Program) DeclName(ent sem.EntityID) string {
	if p.R.Entity(ent).Kind == sem.EntClosure {
		return p.FuncName(ent)
	}
	if id := p.R.DeclID(ent); !id.IsZero() {
		return id.String()
	}
	return p.R.Entity(ent).Name
}
