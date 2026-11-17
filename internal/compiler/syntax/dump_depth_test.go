package syntax_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func TestDumpIndentIsBounded(t *testing.T) {
	const n = 3000
	src := "fn f() -> Int { " + strings.Repeat("{", n) + "1" + strings.Repeat("}", n) + " }\n"
	tree := syntax.Parse(token.NewFile("w.kg", []byte(src)))
	dump := tree.Dump()
	if limit := 200 * len(src); len(dump) > limit {
		t.Fatalf("dump of %d bytes of source is %d bytes", len(src), len(dump))
	}
	for line := range strings.SplitSeq(dump, "\n") {
		if indent := len(line) - len(strings.TrimLeft(line, " ")); indent > 2*syntax.MaxIndentDepth {
			t.Fatalf("indent %d exceeds the bound", indent)
		}
	}
}
