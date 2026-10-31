package sem

import "strings"

// sameArgs ignores a plain-vs-escaping difference in a parameter's fn type: escapeMismatch is the
// specific, sharper-diagnostic check for that, and must still run even if that's the only difference.
func sameArgs(tt *TypeTable, a, b []TypeID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !tt.SameIgnoringEscaping(a[i], b[i]) {
			return false
		}
	}
	return true
}

func sameCallableShape(tt *TypeTable, a, b TypeID) bool {
	if len(tt.Node(a).Args) != len(tt.Node(b).Args) {
		return false
	}
	an, bn := tt.Node(a), tt.Node(b)
	for i, p := range an.Args {
		q := bn.Args[i]
		if tt.RefLifetime(p) == 0 || tt.RefLifetime(q) == 0 {
			p, q = tt.WithRefLifetime(p, 0), tt.WithRefLifetime(q, 0)
		}
		if !tt.SameIgnoringEscaping(p, q) {
			return false
		}
	}
	return tt.SameIgnoringEscaping(an.Elem, bn.Elem)
}

func recvText(k RecvKind) string {
	switch k {
	case RecvMut:
		return "mut self"
	case RecvMove:
		return "move self"
	}
	return "self"
}

func (r *Result) reqText(name string, recv RecvKind, sig TypeID) string {
	n := r.Types.Node(sig)
	var sb strings.Builder
	sb.WriteString("fn " + name + "(")
	if recv != RecvNone {
		sb.WriteString(recvText(recv))
	}
	for i, a := range n.Args {
		if i > 0 || recv != RecvNone {
			sb.WriteString(", ")
		}
		sb.WriteString(r.TypeString(a))
	}
	sb.WriteString(") -> " + r.TypeString(n.Elem))
	return sb.String()
}
