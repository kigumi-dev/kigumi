package syntax_test

import (
	"os"
	"regexp"
	"testing"

	"kigumi/internal/diag"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

const grammarPath = "../../editors/vscode/syntaxes/kigumi.tmLanguage.json"

func TestGrammarKnowsEveryKeyword(t *testing.T) {
	grammar, err := os.ReadFile(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range append(token.Keywords(), token.Contextual...) {
		if !regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`).Match(grammar) {
			t.Errorf("grammar does not mention keyword %q", word)
		}
	}
}

func TestGrammarCoversLexerCorpus(t *testing.T) {
	grammar, err := os.ReadFile(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name        string
		src         string
		want        token.Kind
		grammarHint string
	}{
		{"hash comment", "# a comment\n", token.Comment, `#.*\$`},
		{"nested block comment", "/* outer /* inner */ still outer */\n", token.BlockComment, `block-comment`},
		{"lifetime", "'a\n", token.Lifetime, `lifetime`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bag := &diag.Bag{}
			toks := syntax.Lex([]byte(c.src), bag)
			if bag.Len() != 0 {
				t.Fatalf("lexer rejected %q: %d diagnostics", c.src, bag.Len())
			}
			found := false
			for _, tk := range toks {
				if tk.Kind == c.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("lexer did not emit a %s token for %q", c.want, c.src)
			}
			if ok, _ := regexp.Match(c.grammarHint, grammar); !ok {
				t.Errorf("grammar has no pattern matching %q for %s", c.grammarHint, c.name)
			}
		})
	}
	t.Run("hyphenated import path", func(t *testing.T) {
		if !regexp.MustCompile(`A-Za-z0-9_/\.-|A-Za-z0-9_/-\.|A-Za-z0-9_\./-`).Match(grammar) {
			t.Errorf("grammar's import path pattern must allow a hyphen")
		}
	})
	t.Run("namespaced attribute", func(t *testing.T) {
		tree := syntax.Parse(token.NewFile("x.kg", []byte("@json.name(\"x\")\nfn f() -> Int {\n    0\n}\n")))
		if len(tree.Diags) != 0 {
			t.Fatalf("parser rejected a namespaced attribute: %s", diag.RenderAll(tree.File, tree.Diags))
		}
		if !regexp.MustCompile(`@\[A-Za-z_\]\[A-Za-z0-9_\]\*.*\\\.`).Match(grammar) {
			t.Errorf("grammar's attribute pattern must allow a dotted namespace")
		}
	})
}
