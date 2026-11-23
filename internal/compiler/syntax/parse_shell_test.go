package syntax_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// Shell literals may not contain an empty pipeline stage or a redirect whose
// target starts with another operator, since the runtime would silently drop
// the stage (ARG-6).
func TestShellLiteralRejectsEmptyOperands(t *testing.T) {
	cases := []struct {
		lit  string
		want string
	}{
		{`$"echo hello |"`, "pipeline stage needs a command"},
		{`$"| echo world"`, "pipeline stage needs a command"},
		{`$"echo a | | cat"`, "pipeline stage needs a command"},
		{`$"echo a |"`, "pipeline stage needs a command"},
		{`$"echo > > /tmp/out"`, "redirect needs a target"},
		{`$"echo >> < /tmp/out"`, "redirect needs a target"},
		{`$"cat <"`, "redirect needs a target"},
		{`$"echo >"`, "redirect needs a target"},
		{`$"echo > ;"`, "redirect needs a target"},
	}
	for _, c := range cases {
		t.Run(strings.Trim(c.lit, `$"`), func(t *testing.T) {
			tree := syntax.Parse(token.NewFile("shell.kg", []byte("let p = "+c.lit+"\n")))
			if !shellDiagContains(tree, c.want) {
				t.Fatalf("no %q diagnostic; got %v", c.want, diagMessages(tree))
			}
		})
	}
}

// Valid pipelines and redirects, including fd duplication, must stay accepted.
func TestShellLiteralAcceptsCompleteOperands(t *testing.T) {
	cases := []string{
		`$"echo a | grep b | wc -l"`,
		`$"cat < in > out 2>&1"`,
		`$"printf x >> out"`,
		`$"echo ''"`,
		`$""`,
	}
	for _, lit := range cases {
		t.Run(strings.Trim(lit, `$"`), func(t *testing.T) {
			tree := syntax.Parse(token.NewFile("shell.kg", []byte("let p = "+lit+"\n")))
			if len(tree.Diags) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagMessages(tree))
			}
		})
	}
}

func shellDiagContains(tree *syntax.Tree, want string) bool {
	for _, d := range tree.Diags {
		if strings.Contains(d.Msg, want) {
			return true
		}
	}
	return false
}

func diagMessages(tree *syntax.Tree) []string {
	msgs := make([]string, len(tree.Diags))
	for i, d := range tree.Diags {
		msgs[i] = d.Msg
	}
	return msgs
}
