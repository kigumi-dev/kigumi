package vm

import (
	"strings"

	"kigumi/internal/compiler/sem"
)

// displayRecord is `P { x: 1, y: 2 }`, or `(1, 2)` for a tuple; hashing keeps the bare `P { 1, 2 }` form.
func (m *Machine) displayRecord(sb *strings.Builder, v *obj, canon bool) {
	if !canon && m.isStdMap(v) {
		m.displayMap(sb, v)
		return
	}
	name := m.r.Entity(v.ent).Name
	fields := m.r.TypeDecl(v.ent).Fields
	named := !canon && len(fields) == len(v.fields)
	private := m.r.Packages[m.r.Entity(v.ent).Pkg].Path == "std/secret"
	if named && isTupleName(name) {
		sb.WriteString("(")
		for i, f := range v.fields {
			if i > 0 {
				sb.WriteString(", ")
			}
			m.displayIntoCanon(sb, f, canon)
		}
		sb.WriteString(")")
		return
	}
	sb.WriteString(name)
	sb.WriteString(" {")
	for i, f := range v.fields {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(" ")
		if named {
			sb.WriteString(m.r.Entity(fields[i]).Name)
			sb.WriteString(": ")
			if private && m.r.Entity(fields[i]).Vis.Level == sem.VisPrivate {
				sb.WriteString("<private>")
				continue
			}
		}
		m.displayIntoCanon(sb, f, canon)
	}
	sb.WriteString(" }")
}

func isTupleName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Tuple")
	if !ok || rest == "" {
		return false
	}
	for _, c := range rest {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (m *Machine) isStdMap(v *obj) bool {
	e := m.r.Entity(v.ent)
	return e.Name == "Map" && len(v.fields) == 5 && m.r.Packages[e.Pkg].Path == "std/map"
}

// displayMap is `Map { k: v, ... }` in insertion order; the hash index is not part of what a map is.
func (m *Machine) displayMap(sb *strings.Builder, v *obj) {
	keys, vals := deref(v.fields[0]), deref(v.fields[1])
	sb.WriteString("Map {")
	for i := range keys.fields {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(" ")
		m.displayInto(sb, keys.fields[i])
		sb.WriteString(": ")
		m.displayInto(sb, vals.fields[i])
	}
	if len(keys.fields) > 0 {
		sb.WriteString(" ")
	}
	sb.WriteString("}")
}
