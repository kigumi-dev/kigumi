package llgen

import (
	"strings"

	"kigumi/internal/sem"
)

// desc names a type descriptor after the package and type it describes,
// so the symbol does not depend on entity numbering.
func (e *emitter) desc(ent sem.EntityID) string {
	if name, ok := e.descs[ent]; ok {
		return name
	}
	name := "@\".type." + e.qualifiedName(ent) + "\""
	e.descs[ent] = name
	return name
}

// primDesc names the descriptor of a type with no declaration after its key.
func (e *emitter) primDesc(t sem.TypeID) string {
	key := string(e.r.TypeKey(t))
	if name, ok := e.primDescs[key]; ok {
		return name
	}
	if e.primDescs == nil {
		e.primDescs = map[string]string{}
	}
	name := "@\".type.prim." + key + "\""
	e.primDescs[key] = name
	return name
}

func (e *emitter) vdesc(v sem.EntityID) string {
	if name, ok := e.vdescs[v]; ok {
		return name
	}
	name := "@\".variant." + e.qualifiedName(e.r.Entity(v).Parent) + "." + e.r.Entity(v).Name + "\""
	e.vdescs[v] = name
	return name
}

// qualifiedName is `<package path>.<name>` for a declaration; universe
// declarations (Option, String) have no package.
func (e *emitter) qualifiedName(ent sem.EntityID) string {
	if id := e.r.DeclID(ent); !id.IsZero() && id.Package != "" {
		return id.String()
	}
	return "universe." + e.r.Entity(ent).Name
}

// subSymbol derives a companion global (`.fields`, `.methods`) from a
// possibly quoted symbol name, keeping the quotes around the whole name.
func subSymbol(name, suffix string) string {
	if strings.HasPrefix(name, "@\"") && strings.HasSuffix(name, "\"") {
		return name[:len(name)-1] + "." + suffix + "\""
	}
	return name + "." + suffix
}
