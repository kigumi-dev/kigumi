package lsp_test

import "testing"

func TestRenameParameterPassedByName(t *testing.T) {
	source := "fn k(first: Int, other: Int) -> Int { first + other }\n" +
		"let a = k(first: 1, other: 2)\n" +
		"let b = k(3, other: 4)\n" +
		"print \"${a} ${b}\"\n"
	want := "fn k(second: Int, other: Int) -> Int { second + other }\n" +
		"let a = k(second: 1, other: 2)\n" +
		"let b = k(3, other: 4)\n" +
		"print \"${a} ${b}\"\n"
	for _, at := range [][2]uint32{{0, 5}, {0, 39}, {1, 12}} {
		got, err := renameAt(t, source, at[0], at[1], "second")
		if err != nil {
			t.Fatalf("rename at %v: %v", at, err)
		}
		if got != want {
			t.Fatalf("rename at %v:\n%s", at, got)
		}
	}
}

func TestRenameShorthandRecordPattern(t *testing.T) {
	source := "type P = { first Int, other Int }\n" +
		"fn g(p: P) -> Int { match p { P { first, other } -> first + other } }\n" +
		"print \"${g(P { first: 3, other: 4 })}\"\n"
	cases := []struct {
		name      string
		line, col uint32
		want      string
	}{
		{"field", 0, 11, "type P = { second Int, other Int }\n" +
			"fn g(p: P) -> Int { match p { P { second: first, other } -> first + other } }\n" +
			"print \"${g(P { second: 3, other: 4 })}\"\n"},
		{"binding in arm", 1, 53, "type P = { first Int, other Int }\n" +
			"fn g(p: P) -> Int { match p { P { first: second, other } -> second + other } }\n" +
			"print \"${g(P { first: 3, other: 4 })}\"\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := renameAt(t, source, c.line, c.col, "second")
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("got:\n%s", got)
			}
		})
	}
}
