package mir

import (
	"fmt"
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

func (s *specializer) name(gen sem.EntityID, args []sem.TypeID, wit []sem.WitnessArg) (string, *nameHolder) {
	return s.instanceName(s.p.funcName(gen), args, wit)
}

// nameHolder remembers who took a name so a later witness-only twin can rename both.
type nameHolder struct {
	fn  *Func
	wit []sem.WitnessArg
}

func (s *specializer) instanceName(base string, args []sem.TypeID, wit []sem.WitnessArg) (string, *nameHolder) {
	plain := base + "[" + s.argsText(args) + "]"
	h := &nameHolder{wit: wit}
	prev, taken := s.names[plain]
	if !taken {
		s.names[plain] = h
		return plain, h
	}
	if prev != nil && prev.fn != nil && prev.fn.Name == plain {
		prev.fn.Name = plain + s.witnessText(prev.wit)
		s.names[prev.fn.Name] = prev
	}
	suffixed := plain + s.witnessText(wit)
	name := suffixed
	for n := 2; ; n++ {
		if _, dup := s.names[name]; !dup {
			break
		}
		name = fmt.Sprintf("%s#%d", suffixed, n)
	}
	s.names[name] = h
	return name, h
}

func (s *specializer) witnessText(wit []sem.WitnessArg) string {
	if len(wit) == 0 {
		return ""
	}
	parts := make([]string, len(wit))
	for i, w := range wit {
		parts[i] = s.p.R.WitnessArgKey(w)
	}
	return "<" + strings.Join(parts, ",") + ">"
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

func (s *specializer) typeName(t sem.TypeID) string { return string(s.p.R.TypeKey(t)) }
