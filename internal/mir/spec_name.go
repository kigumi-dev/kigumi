package mir

import (
	"fmt"
	"strconv"
	"strings"

	"kigumi/internal/sem"
)

// Call sites agreeing on callee, type arguments and witnesses share one body.
func (s *specializer) key(gen sem.EntityID, args []sem.TypeID, wit []sem.WitnessArg) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d", gen)
	for _, a := range args {
		fmt.Fprintf(&sb, ",%d", a)
	}
	sb.WriteString("|")
	writeWitnesses(&sb, wit)
	return sb.String()
}

func writeWitnesses(sb *strings.Builder, ws []sem.WitnessArg) {
	for _, w := range ws {
		fmt.Fprintf(sb, "(%d:%d", w.Kind, w.Ent)
		writeWitnesses(sb, w.Args)
		sb.WriteString(")")
	}
}

func (s *specializer) name(gen sem.EntityID, args []sem.TypeID) string {
	base := s.p.funcName(gen)
	if set := s.p.R.Fn(gen).Set; set != 0 && len(s.p.R.Overloads[set].Members) > 1 {
		for i, m := range s.p.R.Overloads[set].Members {
			if m == gen {
				base += "#" + strconv.Itoa(i)
			}
		}
	}
	return s.instanceName(base, args)
}

func (s *specializer) instanceName(base string, args []sem.TypeID) string {
	name := base + "[" + s.argsText(args) + "]"
	for n := 2; s.names[name]; n++ {
		name = fmt.Sprintf("%s[%s]#%d", base, s.argsText(args), n)
	}
	s.names[name] = true
	return name
}

func (s *specializer) argsText(args []sem.TypeID) string {
	var parts []string
	for _, a := range args {
		if a != 0 {
			parts = append(parts, s.typeName(a))
		}
	}
	return strings.Join(parts, ",")
}

func (s *specializer) typeName(t sem.TypeID) string {
	r := s.p.R
	n := r.Types.Node(t)
	switch n.Kind {
	case sem.KNamed, sem.KIface:
		e := r.Entity(n.Ent)
		name := e.Name
		if path := r.Packages[e.Pkg].Path; path != "" {
			name = path + "." + name
		}
		if len(n.Args) > 0 {
			name += "[" + s.argsText(n.Args) + "]"
		}
		return name
	case sem.KClosure:
		return s.p.funcName(n.Ent)
	case sem.KRef:
		if r.Types.RefMut(t) {
			return "&mut(" + s.typeName(n.Elem) + ")"
		}
		return "&" + s.typeName(n.Elem)
	case sem.KFn:
		return "fn(" + s.argsText(n.Args) + ")->" + s.typeName(n.Elem)
	}
	return strings.ReplaceAll(r.TypeString(t), " ", "")
}
