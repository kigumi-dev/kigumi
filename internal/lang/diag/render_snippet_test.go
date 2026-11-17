package diag_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

func TestRenderClipsLongLines(t *testing.T) {
	for _, unit := range []string{"x", "名前", "\u0301", "\t", "\x1b"} {
		for _, pos := range []int{0, 1000000, 2000000} {
			prefix := strings.Repeat(unit, pos)
			src := prefix + "bad" + strings.Repeat(unit, 2000000-pos)
			f := token.NewFile("large.kg", []byte(src))
			d := diag.Errorf(diag.At(f, token.Span{Start: token.Pos(len(prefix)), End: token.Pos(len(prefix) + 3)}), "undefined")
			got := diag.Render(f, d)
			if len(got) > 4096 || !utf8.ValidString(got) || !strings.Contains(got, "bad") || !strings.Contains(got, "^^^") {
				t.Fatalf("unit %q at %d: diagnostic length %d, valid UTF-8 %v", unit, pos, len(got), utf8.ValidString(got))
			}
		}
	}
}
