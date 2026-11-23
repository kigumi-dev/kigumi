package printer_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/printer"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// TestBlockDocCommentsSurviveFormatting pins block-level `///` comments:
// standalone lines and trailing comments must keep their place, formatting
// must be idempotent, and a doc comment before `}` must not become E2199.
func TestBlockDocCommentsSurviveFormatting(t *testing.T) {
	cases := []struct{ name, src string }{
		{
			"standalone",
			"fn f() -> Int {\n    /// keep this documentation comment\n    let x = 1\n    x\n}\n",
		},
		{
			"trailing",
			"fn f() -> Int {\n    let x = 1 /// trailing doc comment\n    x\n}\n",
		},
		{
			"standalone and trailing",
			"fn f() -> Int {\n    /// keep this documentation comment\n    let x = 1 /// trailing doc comment\n    x\n}\n",
		},
		{
			"before closing brace",
			"fn f() {\n    let x = 1\n    /// last doc comment\n}\n",
		},
		{
			"only doc in block",
			"fn f() {\n    /// only doc comment\n}\n",
		},
		{
			"nested blocks",
			"fn f() -> Int {\n    /// outer doc\n    for i in 0..3 {\n        /// inner doc\n        let y = i /// inner trailing\n        y\n    } /// loop trailing\n    0\n}\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tree := syntax.Parse(token.NewFile("a", []byte(c.src)))
			assertNoSyntaxErrors(t, tree.Diags)
			once := printer.Print(tree)
			if once != c.src {
				t.Fatalf("formatting changed canonical source\n--- got ---\n%s--- want ---\n%s", once, c.src)
			}
			reparsed := syntax.Parse(token.NewFile("b", []byte(once)))
			assertNoSyntaxErrors(t, reparsed.Diags)
			if twice := printer.Print(reparsed); twice != once {
				t.Fatalf("formatting is not idempotent\n--- once ---\n%s--- twice ---\n%s", once, twice)
			}
			for _, line := range strings.Split(c.src, "\n") {
				i := strings.Index(line, "///")
				if i < 0 {
					continue
				}
				comment := strings.TrimSpace(line[i:])
				if got := strings.Count(once, comment); got != 1 {
					t.Errorf("doc comment %q printed %d times:\n%s", comment, got, once)
				}
			}
		})
	}
}

func assertNoSyntaxErrors(t *testing.T, diags []diag.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.Severity == diag.Error {
			t.Fatalf("unexpected syntax error: %v", d)
		}
	}
}
