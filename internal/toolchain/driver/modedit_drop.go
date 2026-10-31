package driver

import (
	"bytes"
	"strings"

	"kigumi/internal/lang/token"
)

// dropRecords removes the matching records and, when withComments is set, the comment lines directly above them unless a kept record follows on the very next line (then the comment heads the group), and collapses the blank line a removal would leave doubled.
func dropRecords(f *token.File, recs []record, withComments bool, match func(record) bool) ([]byte, bool) {
	lines := strings.SplitAfter(string(f.Src), "\n")
	drop := map[int]bool{}
	keptAt := map[int]bool{}
	for _, r := range recs {
		if !match(r) {
			keptAt[f.Line(r.Span.Start)] = true
		}
	}
	for _, r := range recs {
		if !match(r) {
			continue
		}
		first := f.Line(r.Span.Start)
		headsKept := keptAt[f.Line(r.Span.End-1)+1]
		for withComments && !headsKept && first > 1 && strings.HasPrefix(strings.TrimSpace(lines[first-2]), "//") {
			first--
		}
		for line := first; line <= f.Line(r.Span.End-1); line++ {
			drop[line] = true
		}
	}
	if len(drop) == 0 {
		return nil, false
	}
	var out bytes.Buffer
	var prevBlank, dirty bool
	for i, text := range lines {
		if drop[i+1] {
			dirty = true
			continue
		}
		blank := strings.TrimSpace(text) == ""
		if blank && dirty && (prevBlank || out.Len() == 0) {
			continue
		}
		out.WriteString(text)
		prevBlank, dirty = blank, false
	}
	if dirty {
		return append(bytes.TrimRight(out.Bytes(), "\n"), '\n'), true
	}
	return out.Bytes(), true
}
