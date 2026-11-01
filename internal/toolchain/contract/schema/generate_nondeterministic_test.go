package schema

import (
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
)

type flipGenerator struct{ runs int }

func (*flipGenerator) ID() GeneratorID { return "derive.flip" }

func (g *flipGenerator) Generate(*Declaration) (*GeneratedUnit, error) {
	g.runs++
	return &GeneratedUnit{Text: fmt.Sprintf("// run %d\n", g.runs)}, nil
}

func TestGenerateDeriveFailsClosedOnNondeterminism(t *testing.T) {
	old := generators["Eq"]
	generators["Eq"] = &flipGenerator{}
	t.Cleanup(func() { generators["Eq"] = old })
	src := "type Point = {\n    pub x Int\n}\n\nfn same(a: Point, b: Point) -> Bool {\n    a == b\n}\n"
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../std"), main)}, GenerateDerive)
	out := res.Render()
	if !strings.Contains(out, "E1430") || !strings.Contains(out, "two runs over the same declaration produced different source") {
		t.Fatalf("expected E1430 naming the disagreement:\n%s", out)
	}
}
