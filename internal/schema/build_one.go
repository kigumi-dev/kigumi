package schema

import "kigumi/internal/sem"

// BuildOne bypasses Build's module-scope filter; a derive generator calls
// it for the one entity its DeriveRequest names.
func BuildOne(res *sem.Result, ent sem.EntityID) *Declaration {
	if ent == 0 {
		return nil
	}
	e := res.Entity(ent)
	if e.File == 0 {
		return nil
	}
	t := res.Tree(e.File)
	pkg := ""
	if int(e.Pkg) < len(res.Packages) {
		pkg = res.Packages[e.Pkg].Path
	}
	decl, _, ok := buildDeclaration(res, t, e.Node, pkg)
	if !ok {
		return nil
	}
	return decl
}
