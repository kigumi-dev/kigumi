package driver_test

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	irItemName     = regexp.MustCompile(`^(?:define|declare) [^@]*@("[^"]*"|[-\w$.]+)\(|^@("[^"]*"|[-\w$.]+) = |^%("[^"]*"|[-\w$.]+) = type `)
	irGlobalRef    = regexp.MustCompile(`@("[^"]*"|[-\w$.]+)`)
	irTypeRef      = regexp.MustCompile(`%("[^"]*"|[-\w$.]+)`)
	irNumbered     = regexp.MustCompile(`^\.[a-z]+\.?\d+`)
	irStdLine      = regexp.MustCompile(`(std/[-\w/.]+\.kg):\d+`)
	irAbiHash      = regexp.MustCompile(`kgabi1:[0-9a-f]+`)
	irMaskedLength = regexp.MustCompile(`\[\d+ x i8\] (c"[^"]*:<line>)`)
)

// irItem is one top-level entity of an IR module: a type ('t'), declaration ('d'), global ('g') or
// function ('f').
type irItem struct {
	kind  byte
	name  string
	lines []string
}

// parseIRItems lists the module's items in order and indexes them as `@name` or `%name`.
func parseIRItems(ir string) ([]*irItem, map[string]*irItem) {
	lines := strings.Split(ir, "\n")
	var items []*irItem
	byName := map[string]*irItem{}
	for i := 0; i < len(lines); i++ {
		m := irItemName.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		it, prefix := &irItem{lines: lines[i : i+1]}, "@"
		switch {
		case m[1] != "" && strings.HasPrefix(lines[i], "define"):
			end := i
			for end < len(lines) && lines[end] != "}" {
				end++
			}
			it.kind, it.name, it.lines = 'f', unquoteSymbol(m[1]), lines[i:end+1]
			i = end
		case m[1] != "":
			it.kind, it.name = 'd', unquoteSymbol(m[1])
		case m[2] != "":
			it.kind, it.name = 'g', unquoteSymbol(m[2])
		default:
			it.kind, it.name, prefix = 't', unquoteSymbol(m[3]), "%"
		}
		items = append(items, it)
		byName[prefix+it.name] = it
	}
	return items, byName
}

// numberedRenamer renames the numbered private globals (`.s12`, `.constval.3`) a text reaches and
// masks std line numbers and the ABI hash.
type numberedRenamer struct {
	globals map[string]string
	names   map[string]string
	order   []string
}

func (r *numberedRenamer) rewrite(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		l = irGlobalRef.ReplaceAllStringFunc(l, func(ref string) string {
			name := unquoteSymbol(ref[1:])
			if _, ok := r.globals[name]; !ok {
				return ref
			}
			if _, ok := r.names[name]; !ok {
				r.names[name] = ".p" + strconv.Itoa(len(r.order))
				r.order = append(r.order, name)
			}
			return "@" + r.names[name]
		})
		l = irStdLine.ReplaceAllString(l, "$1:<line>")
		l = irMaskedLength.ReplaceAllString(l, "[_ x i8] $1")
		out[i] = irAbiHash.ReplaceAllString(l, "kgabi1:<hash>")
	}
	return out
}

func unquoteSymbol(s string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, `"`), `"`)
}
