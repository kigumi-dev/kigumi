package cheader

import (
	"fmt"
	"strings"
)

func render(opaques, records, aliases, fns, consts []item) string {
	var b strings.Builder
	if len(opaques) > 0 || len(fns) > 0 {
		b.WriteString("extern(C) {\n")
		for _, t := range opaques {
			fmt.Fprintf(&b, "    pub %s\n", t.text)
		}
		if len(opaques) > 0 && len(fns) > 0 {
			b.WriteString("\n")
		}
		for _, f := range fns {
			if f.link != "" {
				fmt.Fprintf(&b, "    @linkName(%q)\n", f.link)
			}
			fmt.Fprintf(&b, "    pub %s\n", f.text)
		}
		b.WriteString("}\n")
	}
	for _, group := range [][]item{records, aliases, consts} {
		for _, it := range group {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "pub %s\n", it.text)
		}
	}
	return b.String()
}

func dedupe(s []string) []string {
	var out []string
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}
