package mir

import (
	"fmt"
	"strconv"
	"strings"

	"kigumi/internal/compiler/sem"
)

func (p *Program) FuncName(ent sem.EntityID) string {
	e := p.R.Entity(ent)
	switch e.Kind {
	case sem.EntImplicitMain:
		return "main"
	case sem.EntTest:
		// A test's name is an arbitrary string literal; keep it out of the
		// LLVM symbol grammar and append the entity so two names that
		// sanitize alike still get distinct symbols.
		return "test." + symbolText(e.Name) + "." + strconv.Itoa(int(ent))
	case sem.EntClosure:
		return fmt.Sprintf("%s.closure%d", p.FuncName(e.Parent), p.R.ClosureOrdinal(ent))
	}
	if id := p.R.DeclID(ent); !id.IsZero() {
		return id.String()
	}
	return p.R.Packages[e.Pkg].Path + "." + e.Name
}

// symbolText keeps the characters an unquoted symbol may carry; everything else
// becomes `_`, including the quotes and spaces of a test name.
func symbolText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '_' || r == '.' || r == '$' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
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
