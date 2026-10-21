package vm

import "strings"

// displayRecord is `P { x: 1, y: 2 }`, or `(1, 2)` for a tuple; hashing keeps the bare `P { 1, 2 }` form.
func (m *Machine) displayRecord(sb *strings.Builder, v *obj, canon bool) {
	name := m.r.Entity(v.ent).Name
	fields := m.r.TypeDecl(v.ent).Fields
	named := !canon && len(fields) == len(v.fields)
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
