package sem_test

import (
	"errors"
	"strings"
	"testing"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

const derivableEqSrc = `
type Point = {
    pub x Int
}

fn useEq(a: Point, b: Point) -> Bool {
    a == b
}
`

// fakeGenError stands in for internal/schema's GenError without importing
// schema, keeping this a pure sem test of CheckWith's code-selection wiring.
type fakeGenError struct{ kind string }

func (e *fakeGenError) Error() string    { return "fake generator failure: " + e.kind }
func (e *fakeGenError) DiagKind() string { return e.kind }

func checkDerivableEq(t *testing.T, gen sem.DeriveGenerator) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(derivableEqSrc)))
	main.Files = []*syntax.Tree{main.Entry}
	return sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, gen)
}

func TestDeriveGeneratorFailedReachesE1430(t *testing.T) {
	res := checkDerivableEq(t, func(*sem.Result, sem.DeriveRequest) (string, error) {
		return "", errors.New("boom")
	})
	out := res.Render()
	if !strings.Contains(out, "E1430") {
		t.Fatalf("expected E1430:\n%s", out)
	}
}

func TestDeriveGeneratorNoSchemaReachesE1431(t *testing.T) {
	res := checkDerivableEq(t, func(*sem.Result, sem.DeriveRequest) (string, error) {
		return "", &fakeGenError{kind: "no-schema"}
	})
	out := res.Render()
	if !strings.Contains(out, "E1431") {
		t.Fatalf("expected E1431:\n%s", out)
	}
}

func TestDeriveGeneratorNondeterministicReachesE1433(t *testing.T) {
	res := checkDerivableEq(t, func(*sem.Result, sem.DeriveRequest) (string, error) {
		return "", &fakeGenError{kind: "nondeterministic"}
	})
	out := res.Render()
	if !strings.Contains(out, "E1433") {
		t.Fatalf("expected E1433:\n%s", out)
	}
}
