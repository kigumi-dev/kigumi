package lsp

import (
	"testing"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// TestUnusedImportFixesKeepComments checks that pruning a selected import list
// deletes only the unused names and a separator comma, so comments inside the
// braces survive verbatim.
func TestUnusedImportFixesKeepComments(t *testing.T) {
	for _, tc := range []struct {
		name   string
		src    string
		unused []string
		want   string
	}{
		{
			name:   "block comment before the last name",
			src:    "import { a, /* keep */ b } from std/x\n",
			unused: []string{"b"},
			want:   "import { a /* keep */ } from std/x\n",
		},
		{
			name:   "block comment before the first name",
			src:    "import { /* keep */ a, b } from std/x\n",
			unused: []string{"a"},
			want:   "import { /* keep */ b } from std/x\n",
		},
		{
			name:   "line comment on its own line",
			src:    "import {\n    a,\n    // keep this\n    b,\n} from std/x\n",
			unused: []string{"b"},
			want:   "import {\n    a,\n    // keep this\n    \n} from std/x\n",
		},
		{
			name:   "two unused names around a comment",
			src:    "import { a, /* between */ b, c, d } from std/x\n",
			unused: []string{"b", "d"},
			want:   "import { a, /* between */ c } from std/x\n",
		},
		{
			name:   "aliased name",
			src:    "import { foo as f, /* keep */ bar } from std/x\n",
			unused: []string{"bar"},
			want:   "import { foo as f /* keep */ } from std/x\n",
		},
		{
			name:   "all names unused removes the declaration",
			src:    "import { /* both */ a, b } from std/x\nlet x = 1\n",
			unused: []string{"a", "b"},
			want:   "let x = 1\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := token.NewFile("main.kg", []byte(tc.src))
			tree := syntax.Parse(f)
			if len(tree.Diags) != 0 {
				t.Fatalf("parse diagnostics: %+v", tree.Diags)
			}
			spans := importBindingSpans(tree, tc.unused)
			if len(spans) != len(tc.unused) {
				t.Fatalf("found %d unused spans for %v", len(spans), tc.unused)
			}
			var ds []diag.Diagnostic
			for _, sp := range spans {
				ds = append(ds, diag.Diagnostic{Code: "W100", Loc: diag.At(f, sp)})
			}
			got := applyFixes(tc.src, unusedImportFixes(tree, tc.src, ds))
			if got != tc.want {
				t.Fatalf("fixed source:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

// importBindingSpans returns the spans of the selected import bindings named
// in names, matching the node the W100 diagnostic points at.
func importBindingSpans(tree *syntax.Tree, names []string) []token.Span {
	var out []token.Span
	for v := range tree.List(tree.Root).Views() {
		if !v.Is(syntax.ImportDecl) {
			continue
		}
		binding := v.AsImportDecl().Binding()
		if binding.Kind() != syntax.List {
			continue
		}
		list := binding.AsList()
		for i := range list.Len() {
			item := tree.View(list.ItemID(i))
			sp := item.Span()
			if item.Kind() == syntax.ImportAlias {
				sp = item.AsImportAlias().Alias().Span()
			}
			for _, name := range names {
				if tree.TokText(item.Tok().Index()) == name {
					out = append(out, sp)
				}
			}
		}
	}
	return out
}
