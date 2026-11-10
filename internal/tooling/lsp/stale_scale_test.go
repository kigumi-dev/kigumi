package lsp

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/uri"

	"kigumi/internal/lang/token"
)

func bigText(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "let v%d = add(%d, %d)\n", i, i, i+1)
	}
	return b.String()
}

func TestTextShiftScalesToLargeDocuments(t *testing.T) {
	old := bigText(5000)
	lines := strings.SplitAfter(old, "\n")
	lines[5] = "let top = 1\n"
	lines[len(lines)-3] = "let bottom = 2\n"
	cur := strings.Join(lines, "")

	start := time.Now()
	sh := newTextShift([]byte(old), []byte(cur))
	for p := 0; p < len(old); p += 7 {
		sh.toCur(token.Pos(p))
		sh.toOld(token.Pos(p % len(cur)))
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("building and querying the shift of a 5000 line document took %v", d)
	}
	if len(sh.segs) > 12 {
		t.Fatalf("two edited lines should give a handful of segments, got %d", len(sh.segs))
	}
}

func TestShiftIsComputedOncePerDocumentVersion(t *testing.T) {
	root := writeModule(t, map[string]string{"main.kg": staleSrc})
	s, ctx, _ := startServer(t, root, time.Hour)
	path := filepath.Join(root, "main.kg")
	doc := uri.File(path)
	openDoc(s, ctx, doc, 1, staleSrc)
	editDoc(s, ctx, doc, 2, strings.Replace(staleSrc, "// header", "// head", 1))
	snap := s.last[s.primaryRoot()]
	f := snap.fileOf(path).File
	_, _, _ = s.docShift(snap, f)
	first := s.shifts[doc]
	s.docShift(snap, f)
	if first == nil || s.shifts[doc] != first {
		t.Fatal("the shift was rebuilt for the same snapshot and document text")
	}
	editDoc(s, ctx, doc, 3, strings.Replace(staleSrc, "// header", "// heading", 1))
	s.docShift(snap, f)
	if s.shifts[doc] == first {
		t.Fatal("a new document version must replace the cached shift")
	}
}

func TestDiffLinesReproducesTheNewText(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for range 500 {
		old := randomText(rng, 5+rng.Intn(25))
		cur := []byte(old)
		for range 1 + rng.Intn(4) {
			cur = editRandomLine(rng, cur)
		}
		a, b := splitLines([]byte(old)), splitLines(cur)
		var out [][]byte
		i := 0
		for _, h := range diffLines(a, b) {
			out = append(out, a[i:h.a0]...)
			out = append(out, b[h.b0:h.b1]...)
			i = h.a1
		}
		out = append(out, a[i:]...)
		if string(joinLines(out)) != string(cur) {
			t.Fatalf("hunks do not turn old into cur\nold %q\ncur %q", old, cur)
		}
	}
}

func joinLines(lines [][]byte) []byte {
	var out []byte
	for _, l := range lines {
		out = append(out, l...)
	}
	return out
}

func TestSpanInsideEditedRegionWidensToItsLines(t *testing.T) {
	old := "one\ntwo words\nthree\n"
	cur := "one\ntwo changed\nthree\n"
	sh := newTextShift([]byte(old), []byte(cur))
	got := sh.toCurSpan([]byte(cur), token.Span{Start: 9, End: 12})
	if cur[got.Start:got.End] != "two changed" {
		t.Fatalf("a span inside an edited region should cover the edited line, got %q", cur[got.Start:got.End])
	}
	kept := sh.toCurSpan([]byte(cur), token.Span{Start: 0, End: 3})
	if cur[kept.Start:kept.End] != "one" {
		t.Fatalf("an unedited span must keep its text, got %q", cur[kept.Start:kept.End])
	}
}

func TestSpanOfADeletedRegionCollapsesToTheDeletionPoint(t *testing.T) {
	old := "one\nfn gone() {\n}\nthree\n"
	cur := "one\nthree\n"
	sh := newTextShift([]byte(old), []byte(cur))
	got := sh.toCurSpan([]byte(cur), token.Span{Start: 7, End: 11})
	if got.Start != got.End || got.Start != 4 {
		t.Fatalf("a deleted span must collapse to the deletion point (4), got %+v", got)
	}
}
