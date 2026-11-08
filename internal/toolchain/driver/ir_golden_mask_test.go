package driver_test

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	irStdWrapped = regexp.MustCompile(`@"std/[^"]*\$kg"`)
	irValueName  = regexp.MustCompile(`%v\d+`)
)

// pinnedIR keeps what the module's own functions reach, with numbered constants renamed by first use
// and %vN restarted per function, so neither std's constants nor its numbering leak in.
func pinnedIR(ir, module string) string {
	items, byName := parseIRItems(ir)
	kept := map[*irItem]bool{}
	var queue []*irItem
	keep := func(it *irItem) {
		if !kept[it] {
			kept[it] = true
			queue = append(queue, it)
		}
	}
	for _, it := range items {
		if it.kind == 'f' && pinRoot(it, module) {
			keep(it)
		}
	}
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		for _, l := range it.lines {
			for _, m := range irGlobalRef.FindAllStringSubmatch(l, -1) {
				if ref, ok := byName["@"+unquoteSymbol(m[1])]; ok && pinReachable(ref, module) {
					keep(ref)
				}
			}
			for _, m := range irTypeRef.FindAllStringSubmatch(l, -1) {
				if ref, ok := byName["%"+unquoteSymbol(m[1])]; ok {
					keep(ref)
				}
			}
		}
	}
	numbered := map[string]string{}
	var out []string
	for _, it := range items {
		switch {
		case !kept[it]:
		case it.kind == 'g' && irNumbered.MatchString(it.name):
			numbered[it.name] = it.lines[0]
		case it.kind == 'f':
			out = append(out, renumberValues(it.lines)...)
		default:
			out = append(out, it.lines...)
		}
	}
	r := numberedRenamer{globals: numbered, names: map[string]string{}}
	out = r.rewrite(out)
	if len(r.order) > 0 {
		out = append(out, "")
	}
	for i := 0; i < len(r.order); i++ {
		out = append(out, r.rewrite([]string{numbered[r.order[i]]})...)
	}
	return strings.Join(out, "\n") + "\n"
}

// pinRoot picks the module's own functions; glue enters only through a kept function, and a C-named
// export wrapper around a std function belongs to std.
func pinRoot(it *irItem, module string) bool {
	if strings.HasPrefix(it.name, "kg.") || !pinReachable(it, module) {
		return false
	}
	cExport := !strings.Contains(it.name, ".")
	return !cExport || !irStdWrapped.MatchString(strings.Join(it.lines, "\n"))
}

// pinReachable drops std's bodies, except generics instantiated at the module's types, and
// kg_init_immortals, which registers every constant of the linked program, std's included.
func pinReachable(it *irItem, module string) bool {
	switch {
	case it.kind != 'f':
		return true
	case it.name == "kg_init_immortals":
		return false
	case !strings.HasPrefix(it.name, "std/"):
		return true
	}
	_, args, generic := strings.Cut(it.name, "[")
	return generic && strings.Contains(args, module+".")
}

func renumberValues(lines []string) []string {
	names := map[string]string{}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = irValueName.ReplaceAllStringFunc(l, func(v string) string {
			if _, ok := names[v]; !ok {
				names[v] = "%v" + strconv.Itoa(len(names)+1)
			}
			return names[v]
		})
	}
	return out
}
