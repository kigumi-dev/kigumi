package syntax_test

import (
	"testing"

	"kigumi/internal/compiler/syntax"
)

// A `{` or `}` in a doc comment is comment text, not a brace of the
// declaration; counting it would extend the head past its own closing brace
// or cut it short.
func TestDeclHeadIgnoresBracesInDocComments(t *testing.T) {
	src := `pub type R = {
    pub first Int
    /// uses the { placeholder
    hidden Int
    pub shown String
}

pub type S = {
    /// closes } early
    pub kept Int
}

pub const after = 1
`
	tree := parseOK(t, src)
	list := tree.List(tree.Root)
	r := list.At(0)
	want := "pub type R = {\n    pub first Int\n    /// uses the { placeholder\n    hidden Int\n    pub shown String\n}"
	if got := syntax.DeclHead(tree, r.ID()); got != want {
		t.Errorf("DeclHead(R) =\n%s\nwant\n%s", got, want)
	}
	wantPublic := "pub type R = {\n    pub first Int\n    pub shown String\n    // private fields omitted\n}"
	if got := syntax.PublicDeclHead(tree, r.ID()); got != wantPublic {
		t.Errorf("PublicDeclHead(R) =\n%s\nwant\n%s", got, wantPublic)
	}
	s := list.At(1)
	wantS := "pub type S = {\n    /// closes } early\n    pub kept Int\n}"
	if got := syntax.DeclHead(tree, s.ID()); got != wantS {
		t.Errorf("DeclHead(S) =\n%s\nwant\n%s", got, wantS)
	}
	if after := list.At(2); syntax.DeclHead(tree, after.ID()) != "pub const after = 1" {
		t.Errorf("following declaration absorbed: %q", syntax.DeclHead(tree, after.ID()))
	}
}
