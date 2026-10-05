package schema

import "kigumi/internal/sem"

// declID never uses a process-local EntityID ordinal, so reordering the
// same source leaves it unchanged.
func declID(pkg, kind, name string) string {
	return pkg + "#" + kind + ":" + name
}

// memberID extends a declaration's stable ID to one of its params, fields
// or variants.
func memberID(decl string, name string) string {
	return decl + "." + name
}

func declKind(res *sem.Result, e sem.EntityID) (Kind, string) {
	ent := res.Entity(e)
	switch ent.Kind {
	case sem.EntFn:
		if res.Fn(e).Owner != 0 {
			return KindMethod, res.Entity(res.Fn(e).Owner).Name + "." + ent.Name
		}
		return KindFn, ent.Name
	case sem.EntType:
		return KindType, ent.Name
	case sem.EntConst:
		return KindConst, ent.Name
	case sem.EntStatic:
		return KindStatic, ent.Name
	case sem.EntInterface:
		return KindInterface, ent.Name
	}
	return "", ""
}

// declRefOf names ent by the same package+name pair declID hashes.
func declRefOf(res *sem.Result, ent sem.EntityID) *DeclRef {
	if ent == 0 {
		return nil
	}
	e := res.Entity(ent)
	pkg := ""
	if int(e.Pkg) < len(res.Packages) {
		pkg = res.Packages[e.Pkg].Path
	}
	_, name := declKind(res, ent)
	if name == "" {
		name = e.Name
	}
	return &DeclRef{ID: declID(pkg, string(entKindTag(res, ent)), name), Package: pkg, Name: name}
}

func entKindTag(res *sem.Result, ent sem.EntityID) Kind {
	k, _ := declKind(res, ent)
	return k
}

func visibilityOf(level sem.VisLevel) Visibility {
	switch level {
	case sem.VisPub:
		return VisPub
	case sem.VisModule:
		return VisModule
	case sem.VisSuper, sem.VisIn:
		return VisSuper
	default:
		return VisPrivate
	}
}
