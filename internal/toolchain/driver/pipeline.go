package driver

import (
	"fmt"
	"io"
	"os"

	"kigumi/internal/compiler/printer"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// Source is one input file; Path "-" reads standard input.
type Source struct {
	Path string
	Src  []byte
}

func Load(path string) (Source, error) {
	var b []byte
	var err error
	if path == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(path)
	}
	if err != nil {
		return Source{}, err
	}
	return Source{Path: path, Src: b}, nil
}

func (s Source) file() *token.File { return token.NewFile(s.Path, s.Src) }

// Parse returns the tree and the rendered diagnostics.
func Parse(s Source) (*syntax.Tree, string) {
	tree := syntax.Parse(s.file())
	return tree, diagRender(tree)
}

func diagRender(tree *syntax.Tree) string {
	out := diag.RenderAll(tree.File, tree.Diags)
	if tree.DiagsDropped > 0 {
		out += fmt.Sprintf("%s: %d diagnostics omitted (parse diagnostic limit reached)\n\n", tree.File.Name, tree.DiagsDropped)
	}
	return out
}

func hasErrors(tree *syntax.Tree) bool {
	for _, d := range tree.Diags {
		if d.Severity == diag.Error {
			return true
		}
	}
	return false
}

// Format returns the canonical form of s.
func Format(s Source) (string, error) {
	tree, rendered := Parse(s)
	if hasErrors(tree) {
		return "", fmt.Errorf("%s", rendered)
	}
	return printer.Print(tree), nil
}
