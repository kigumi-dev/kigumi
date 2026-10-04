package diag_test

import (
	"strings"
	"testing"

	"kigumi/internal/diag"
	"kigumi/internal/token"
)

func TestRenderCJKCaretAlignment(t *testing.T) {
	src := "let 名 = foo\n"
	f := token.NewFile("a.kg", []byte(src))
	nameStart := strings.Index(src, "foo")
	d := diag.Errorf(diag.At(f, token.Span{Start: token.Pos(nameStart), End: token.Pos(nameStart + 3)}), "undefined")
	got := diag.Render(f, d)
	lines := strings.Split(got, "\n")
	var underline string
	for _, l := range lines {
		if strings.Contains(l, "^") {
			underline = l
		}
	}
	if underline == "" {
		t.Fatalf("no underline in %q", got)
	}
	gutter := "  | "
	rest := underline[len(gutter):]
	leadSpaces := len(rest) - len(strings.TrimLeft(rest, " "))
	// "名" occupies 2 display cells despite being 3 bytes in UTF-8;
	// "let " (4) + "名"(2 cells) + " = " (3) = 9 cells before "foo".
	if leadSpaces != 9 {
		t.Fatalf("caret lead = %d cells, want 9; line %q", leadSpaces, underline)
	}
	if carets := strings.Count(rest, "^"); carets != 3 {
		t.Fatalf("caret width = %d, want 3 for ascii span %q", carets, underline)
	}
}

func TestRenderWideSpanUnderline(t *testing.T) {
	src := "let x = 名前\n"
	f := token.NewFile("a.kg", []byte(src))
	start := strings.Index(src, "名前")
	d := diag.Errorf(diag.At(f, token.Span{Start: token.Pos(start), End: token.Pos(start + len("名前"))}), "undefined")
	got := diag.Render(f, d)
	var underline string
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "^") {
			underline = l
		}
	}
	if carets := strings.Count(underline, "^"); carets != 4 {
		t.Fatalf("caret width = %d, want 4 (2 wide runes x 2 cells); line %q", carets, underline)
	}
}

func TestRenderTabExpansion(t *testing.T) {
	src := "\tfoo\n"
	f := token.NewFile("a.kg", []byte(src))
	d := diag.Errorf(diag.At(f, token.Span{Start: 1, End: 4}), "undefined")
	got := diag.Render(f, d)
	var underline string
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "^") {
			underline = l
		}
	}
	rest := underline[len("  | "):]
	leadSpaces := len(rest) - len(strings.TrimLeft(rest, " "))
	if leadSpaces != 4 {
		t.Fatalf("a leading tab must expand to the next stop of 4, got lead %d in %q", leadSpaces, underline)
	}
}

func TestRenderCombiningMarkZeroWidth(t *testing.T) {
	src := "let é = foo\n" // "é" as e + combining acute accent
	f := token.NewFile("a.kg", []byte(src))
	start := strings.Index(src, "foo")
	d := diag.Errorf(diag.At(f, token.Span{Start: token.Pos(start), End: token.Pos(start + 3)}), "undefined")
	got := diag.Render(f, d)
	var underline string
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "^") {
			underline = l
		}
	}
	rest := underline[len("  | "):]
	leadSpaces := len(rest) - len(strings.TrimLeft(rest, " "))
	// "let " (4) + "e" + combining accent (1 cell total) + " = " (3) = 8
	if leadSpaces != 8 {
		t.Fatalf("combining mark must not add a display cell, lead = %d in %q", leadSpaces, underline)
	}
}
