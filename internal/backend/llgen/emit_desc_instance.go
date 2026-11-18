package llgen

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"kigumi/internal/compiler/sem"
)

// instDesc is the static descriptor of one instantiation whose drop specialize made a body for: a lane shape
// leaves no shared body to run.
type instDesc struct {
	name string
	t    sem.TypeID
}

// simdInstDesc names the static descriptor of concrete instantiation t when its drop runs a body of its own, else
// "null": a descriptor made at runtime from the generic one serves.
func (e *emitter) simdInstDesc(t sem.TypeID) string {
	ent := e.r.Types.Node(t).Ent
	drop := e.r.TypeDecl(ent).Drop
	if drop == 0 || e.p.Desc.Get(e.r, drop, t) == nil {
		return "null"
	}
	t = e.canon(t)
	if d, ok := e.instDescs[t]; ok {
		return d.name
	}
	if e.instDescs == nil {
		e.instDescs = map[sem.TypeID]instDesc{}
	}
	name := "@\".type.inst." + llEscape(e.r.TypeString(t)) + "." + strconv.Itoa(int(t)) + "\""
	e.instDescs[t] = instDesc{name: name, t: t}
	e.desc(ent)
	e.useDescAdapter(drop, t)
	return name
}

// instDescData writes the static instantiation descriptors; each shares its entity's field and variant tables.
func (e *emitter) instDescData(sb *strings.Builder) {
	for _, t := range slices.Sorted(maps.Keys(e.instDescs)) {
		d := e.instDescs[t]
		ent := e.r.Types.Node(d.t).Ent
		x, info, base := e.r.Entity(ent), e.r.TypeDecl(ent), e.desc(ent)
		fields, variants := "null", "null"
		if len(info.Fields) > 0 {
			fields = subSymbol(base, "fields")
		}
		if len(info.Variants) > 0 {
			variants = subSymbol(base, "variants")
		}
		e.irb(sb).GlobalStruct(d.name, rtTypeStruct,
			vptr(e.cstr(e.r.Packages[x.Pkg].Path+"."+x.Name)), vi32(e.typeKind(ent, info)), vi32(len(info.Fields)),
			vptr(fields), vi32(len(info.Variants)), vptr(variants), vptr(e.useDescAdapter(info.Drop, d.t)), vi64(int(e.borrowFieldMask(info))), vi64(int(e.privateFieldMask(info))), vptr(base), vptr("null"))
	}
}
