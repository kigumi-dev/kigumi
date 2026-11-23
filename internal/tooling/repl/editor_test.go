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

// TestEditorEscapeSequences feeds raw CSI and SS3 sequences to the raw editor
// and checks both their editing effect and that no parameter or terminator
// byte leaks into the line buffer.
func TestEditorEscapeSequences(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		hist []string
		want string
	}{
		{"application left arrows", "ab\x1bOD\x1bODX\r", nil, "Xab"},
		{"application home and right", "ab\x1bOH\x1bOCX\r", nil, "aXb"},
		{"application end", "ab\x1bOH\x1bOFX\r", nil, "abX"},
		{"application up recalls history", "\x1bOA\r", []string{"first", "second"}, "second"},
		{"application down clears newest", "\x1bOA\x1bOBX\r", []string{"first", "second"}, "X"},
		{"delete key removes under cursor", "abc\x1b[D\x1b[3~d\r", nil, "abd"},
		{"delete key at end is consumed", "abc\x1b[3~\r", nil, "abc"},
		{"insert key is consumed", "ab\x1b[2~X\r", nil, "abX"},
		{"home tilde", "ab\x1b[1~X\r", nil, "Xab"},
		{"end tilde", "ab\x1b[1~\x1b[4~X\r", nil, "abX"},
		{"function key is consumed", "ab\x1b[15~X\r", nil, "abX"},
		{"modified arrow", "ab\x1b[H\x1b[1;5CX\r", nil, "aXb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := NewHistory("")
			for _, l := range tc.hist {
				h.Add(l)
			}
			var out bytes.Buffer
			ed := &Editor{raw: true, reader: bufio.NewReader(strings.NewReader(tc.in)), out: &out, history: h}
			line, err := ed.ReadLine("> ")
			if err != nil {
				t.Fatalf("ReadLine = %q, %v", line, err)
			}
			if line != tc.want {
				t.Fatalf("ReadLine(%q) = %q; want %q", tc.in, line, tc.want)
			}
		})
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
