package sem

import "strings"

func fillTemplate(template string, fmtArg func(any) string, args []any) string {
	out, _ := fillHoles(template, fmtArg, args, map[string]string{})
	return out
}

// fillCode numbers holes across message and help; a hole left without an
// argument repeats an earlier hole of the same name.
func fillCode(c Code, fmtArg func(any) string, args []any) (msg, help string) {
	seen := map[string]string{}
	msg, used := fillHoles(c.Template, fmtArg, args, seen)
	if c.Help != "" {
		help, _ = fillHoles(c.Help, fmtArg, args[used:], seen)
	}
	return msg, help
}

func fillHoles(template string, fmtArg func(any) string, args []any, seen map[string]string) (string, int) {
	var sb strings.Builder
	used := 0
	rest := template
	for {
		i := strings.IndexByte(rest, '{')
		j := strings.IndexByte(rest, '}')
		if i < 0 || j < i {
			break
		}
		name := rest[i+1 : j]
		val, ok := seen[name]
		if used < len(args) {
			val, ok = fmtArg(args[used]), true
			seen[name] = val
			used++
		}
		sb.WriteString(rest[:i])
		if ok {
			sb.WriteString(val)
		} else {
			sb.WriteString(rest[i : j+1])
		}
		rest = rest[j+1:]
	}
	sb.WriteString(rest)
	return sb.String(), used
}
