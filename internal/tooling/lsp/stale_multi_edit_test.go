package lsp

import (
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/lang/token"
)

const staleSrc = "// header\nfn add(a: Int, b: Int) -> Int {\n    a + b\n}\n\nfn sub(a: Int, b: Int) -> Int {\n    a - b\n}\n\nlet x = add(1, 2)\nlet y = sub(3, 4)\nprint \"${x} ${y}\"\n"

func TestStaleAnswersSurviveTwoDisjointEdits(t *testing.T) {
	root := writeModule(t, map[string]string{"main.kg": staleSrc})
	s, ctx, _ := startServer(t, root, time.Hour)
	doc := uri.File(filepath.Join(root, "main.kg"))
	openDoc(s, ctx, doc, 1, staleSrc)
	lines := strings.Split(staleSrc, "\n")
	lines[1] += " "
	editDoc(s, ctx, doc, 2, strings.Join(lines, "\n"))
	lines[9] += " "
	editDoc(s, ctx, doc, 3, strings.Join(lines, "\n"))

	at := func(line, ch uint32) protocol.TextDocumentPositionParams {
		return protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}, Position: protocol.Position{Line: line, Character: ch}}
	}
	def, _ := s.Definition(ctx, &protocol.DefinitionParams{TextDocumentPositionParams: at(10, 10)})
	loc, ok := def.(*protocol.Location)
	want := protocol.Range{Start: protocol.Position{Line: 5, Character: 3}, End: protocol.Position{Line: 5, Character: 6}}
	if !ok || loc.Range != want {
		t.Fatalf("definition of sub during the debounce: got %+v, want range %+v", def, want)
	}
	if h, _ := s.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: at(9, 8)}); h == nil {
		t.Fatal("hover on add during the debounce returned null")
	}
	inEdit := at(1, 31)
	if h, _ := s.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: inEdit}); h != nil {
		t.Fatalf("hover on text added since the analysis must be null, got %+v", h)
	}
}

func TestTextShiftMapsUneditedBytesExactly(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for range 300 {
		old := randomText(rng, 8+rng.Intn(20))
		cur := []byte(old)
		for range 1 + rng.Intn(3) {
			cur = editRandomLine(rng, cur)
		}
		sh := newTextShift([]byte(old), cur)
		for p := range cur {
			q, ok := sh.toOld(token.Pos(p))
			next, nextOK := sh.toOld(token.Pos(p + 1))
			if ok && nextOK && next == q+1 && old[q] != cur[p] {
				t.Fatalf("cur[%d]=%q maps to old[%d]=%q\nold %q\ncur %q", p, cur[p], q, old[q], old, cur)
			}
		}
	}
}

func randomText(rng *rand.Rand, lines int) string {
	var b strings.Builder
	for i := range lines {
		b.WriteString(strings.Repeat(string(rune('a'+rng.Intn(5))), 1+rng.Intn(6)))
		b.WriteString(strings.Repeat(" ", i%2))
		b.WriteByte('\n')
	}
	return b.String()
}

func editRandomLine(rng *rand.Rand, text []byte) []byte {
	lines := strings.SplitAfter(string(text), "\n")
	i := rng.Intn(len(lines))
	switch rng.Intn(3) {
	case 0:
		lines[i] = "x" + lines[i]
	case 1:
		lines = append(lines[:i], lines[i+1:]...)
	default:
		lines = append(lines[:i], append([]string{"new\n"}, lines[i:]...)...)
	}
	return []byte(strings.Join(lines, ""))
}
