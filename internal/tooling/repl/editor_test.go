package repl

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestEditorNonTerminalReadsPlainLines covers the fallback every platform
// takes when stdin is a pipe: no raw mode, no escape processing, but the same
// prompt and line trimming as PlainLine.
func TestEditorNonTerminalReadsPlainLines(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := io.WriteString(w, "let x\r\nsecond\n"); err != nil {
		w.Close()
		t.Fatal(err)
	}
	w.Close()

	var out bytes.Buffer
	h, _ := NewHistory("")
	ed, err := NewEditor(r, &out, h)
	if err != nil {
		t.Fatal(err)
	}
	if ed.IsTerminal() {
		t.Fatal("pipe stdin must not enter raw mode")
	}
	line, err := ed.ReadLine("> ")
	if err != nil || line != "let x" {
		t.Fatalf("ReadLine = %q, %v", line, err)
	}
	if got := out.String(); got != "> " {
		t.Fatalf("prompt output = %q", got)
	}
	line, err = ed.ReadLine("")
	if err != nil || line != "second" {
		t.Fatalf("second ReadLine = %q, %v", line, err)
	}
	if _, err := ed.ReadLine(""); err != io.EOF {
		t.Fatalf("EOF = %v", err)
	}
}

func TestPlainLineTrimsLineEndings(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"hello\r\n", "hello"},
		{"hello\n", "hello"},
		{"\n", ""},
	} {
		var out strings.Builder
		line, err := PlainLine(bufio.NewReader(strings.NewReader(tc.in)), &out, "> ")
		if err != nil || line != tc.want {
			t.Errorf("PlainLine(%q) = %q, %v; want %q", tc.in, line, err, tc.want)
		}
		if out.String() != "> " {
			t.Errorf("PlainLine(%q) prompt = %q", tc.in, out.String())
		}
	}
}
