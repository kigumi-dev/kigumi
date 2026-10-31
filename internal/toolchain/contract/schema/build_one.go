package schema

import "kigumi/internal/compiler/sem"

// BuildOne bypasses Build's module-scope filter for the one entity a derive
// generator names; a synthesized type (Option, Result, TupleN) builds from sem's typed info, not a syntax tree.
func BuildOne(res *sem.Result, ent sem.EntityID) *Declaration {
	if ent == 0 {
		return nil
	}
	e := res.Entity(ent)
	if e.File == 0 {
		return buildSynthesizedType(res, ent)
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

// builtinPkg is where a predeclared type's derived methods land; derive_gen.go
// (internal/compiler/sem) routes their generated source the same way.
const builtinPkg = "std/prelude"

// buildSynthesizedType builds Option/Result/TupleN's Declaration without a
// syntax tree; it skips Doc/Display/Origin/Availability/Metadata, none of which a derive generator reads.
func buildSynthesizedType(res *sem.Result, ent sem.EntityID) *Declaration {
	e := res.Entity(ent)
	if e.Kind != sem.EntType {
		return nil
	}
	decl := &Declaration{Package: builtinPkg, Kind: KindType, Name: e.Name, Visibility: visibilityOf(e.Vis.Level)}
	info := res.TypeDecl(ent)
	for _, p := range info.Params {
		m := buildMember(res, nil, p, nil)
		m.Const = res.TypeParam(p).IsConst
		decl.Params = append(decl.Params, m)
	}
	for _, f := range info.Fields {
		decl.Fields = append(decl.Fields, buildMember(res, nil, f, nil))
	}
	for _, v := range info.Variants {
		decl.Variants = append(decl.Variants, buildVariant(res, nil, v))
	}
	_, name := declKind(res, ent)
	decl.ID = declID(decl.Package, string(KindType), name)
	return decl
}
