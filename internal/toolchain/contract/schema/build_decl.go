package schema

import (
	"kigumi/internal/base/platform"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// buildDeclaration reports false when d is not one of the kinds the Schema
// IR covers; pkg must already be the owning package, since decl.ID hashes it.
func buildDeclaration(res *sem.Result, t *syntax.Tree, d syntax.NodeID, pkg string) (*Declaration, sem.EntityID, bool) {
	ent := res.File(t).Defs[d]
	if ent == 0 {
		return nil, 0, false
	}
	e := res.Entity(ent)
	decl := &Declaration{
		Package:      pkg,
		Name:         e.Name,
		Visibility:   visibilityOf(e.Vis.Level),
		Display:      syntax.DeclHead(t, d),
		Doc:          syntax.DocText(t, d),
		Origin:       originOf(t, d),
		Availability: availabilityOf(t.File.Name),
	}
	switch e.Kind {
	case sem.EntFn:
		buildFn(res, t, ent, decl)
	case sem.EntType:
		buildType(res, t, ent, decl)
	case sem.EntConst:
		decl.Kind = KindConst
		decl.Metadata = metadataOf(res, t, res.Const(ent).Metadata)
	case sem.EntStatic:
		decl.Kind = KindStatic
		decl.Metadata = metadataOf(res, t, res.Static(ent).Metadata)
	case sem.EntInterface:
		decl.Kind = KindInterface
	default:
		return nil, 0, false
	}
	kind, name := declKind(res, ent)
	decl.Name, decl.ID = name, entityID(res, ent, decl.Package, string(kind), name)
	return decl, ent, true
}

func buildFn(res *sem.Result, t *syntax.Tree, ent sem.EntityID, decl *Declaration) {
	info := res.Fn(ent)
	decl.Kind = KindFn
	if info.Owner != 0 {
		decl.Kind = KindMethod
	}
	decl.Metadata = metadataOf(res, t, info.Metadata)
	decl.Contracts = contractsOf(res, ent)
	decl.ABI = abiOf(res, ent)
	if sig := res.Types.Node(info.Sig); info.Sig != 0 && sig.Kind == sem.KFn {
		if sig.Elem != sem.TyUnit && sig.Elem != sem.TyNever {
			decl.Return = typeRef(res, sig.Elem)
		}
	}
	for _, p := range info.Params {
		decl.Params = append(decl.Params, buildMember(res, t, p, res.Local(p).Metadata))
	}
}

// abiOf names the foreign block ent sits in; nil for a Kigumi-only function.
func abiOf(res *sem.Result, ent sem.EntityID) *ABI {
	info := res.Fn(ent)
	block, name := "", ""
	switch {
	case info.Abi != "":
		block, name = "extern", info.Abi
	case info.Export != "":
		block, name = "export", info.Export
	default:
		return nil
	}
	return &ABI{Block: block, Name: name, Symbol: res.FnLinkNameOf(ent), Naked: info.Naked, Variadic: cVariadic(res, info.Sig)}
}

func cVariadic(res *sem.Result, sig sem.TypeID) bool {
	return sig != 0 && res.Types.Node(sig).Flags&sem.FnCVariadic != 0
}

func buildType(res *sem.Result, t *syntax.Tree, ent sem.EntityID, decl *Declaration) {
	info := res.TypeDecl(ent)
	decl.Kind = KindType
	decl.Metadata = metadataOf(res, t, info.Metadata)
	decl.SchemaDecl = schemaDeclOf(res, ent)
	decl.Form = typeFormName(info.Form)
	decl.Layout, decl.Align = info.Layout, info.Align
	if info.Form == sem.FormDefined {
		decl.Underlying = typeRef(res, info.Underlying)
	}
	for _, p := range info.Params {
		m := buildMember(res, t, p, nil)
		m.Const = res.TypeParam(p).IsConst
		decl.Params = append(decl.Params, m)
	}
	for _, f := range info.Fields {
		decl.Fields = append(decl.Fields, buildMember(res, t, f, res.Field(f).Metadata))
	}
	for _, v := range info.Variants {
		decl.Variants = append(decl.Variants, buildVariant(res, t, v))
	}
}

func buildMember(res *sem.Result, t *syntax.Tree, ent sem.EntityID, meta []sem.MetaRef) *Member {
	e := res.Entity(ent)
	return &Member{ID: memberID(declRefOf(res, e.Parent).ID, e.Name), Name: e.Name, Type: typeRef(res, e.Type), Metadata: metadataOf(res, t, meta),
		Escaping: e.Kind == sem.EntParam && e.Flags&sem.EfEscaping != 0}
}

func buildVariant(res *sem.Result, t *syntax.Tree, v sem.EntityID) *Variant {
	info := res.Variant(v)
	name := res.Entity(v).Name
	variant := &Variant{ID: memberID(declRefOf(res, res.Entity(v).Parent).ID, name), Name: name}
	for i, pt := range info.Payload {
		pname := ""
		if i < len(info.Names) {
			pname = info.Names[i]
		}
		variant.Payload = append(variant.Payload, &Member{ID: memberID(variant.ID, pname), Name: pname, Type: typeRef(res, pt), Metadata: []Metadata{}, Copy: res.IsCopy(pt)})
	}
	return variant
}

// typeFormName names info.Form's shape as a stable IR string.
func typeFormName(form sem.TypeForm) string {
	switch form {
	case sem.FormRecord:
		return "record"
	case sem.FormResource:
		return "resource"
	case sem.FormAdt:
		return "adt"
	case sem.FormOpaque:
		return "opaque"
	case sem.FormDefined:
		return "defined"
	}
	return ""
}

// availabilityOf reads fileName's own platform tag; nil means untagged
// (available on every target), independent of the current build target.
func availabilityOf(fileName string) *TargetAvailability {
	os, arch, tagged := platform.Parse(fileName)
	if !tagged {
		return nil
	}
	return &TargetAvailability{OS: os, Arch: arch}
}
