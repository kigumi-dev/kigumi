package sem_test

import (
	"errors"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
)

const derivableEqSrc = `
type Point = {
    pub x Int
}

fn useEq(a: Point, b: Point) -> Bool {
    a == b
}
`

// fakeGenError stands in for internal/toolchain/contract/schema's GenError without importing
// schema, keeping this a pure sem test of CheckWith's code-selection wiring.
type fakeGenError struct{ kind string }

func (e *fakeGenError) Error() string    { return "fake generator failure: " + e.kind }
func (e *fakeGenError) DiagKind() string { return e.kind }

func checkDerivableEq(t *testing.T, gen sem.DeriveGenerator) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(derivableEqSrc)))
	main.Files = []*syntax.Tree{main.Entry}
	return sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../std"), main)}, gen)
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

func TestDeriveGeneratorOtherKindsReachE1430(t *testing.T) {
	for _, kind := range []string{"failed", "no-schema", "nondeterministic"} {
		res := checkDerivableEq(t, func(*sem.Result, sem.DeriveRequest) (string, error) {
			return "", &fakeGenError{kind: kind}
		})
		if out := res.Render(); !strings.Contains(out, "E1430") {
			t.Errorf("kind %q: expected E1430:\n%s", kind, out)
		}
	}
}

func TestDeriveGeneratorUnsupportedReachesE1432(t *testing.T) {
	res := checkDerivableEq(t, func(*sem.Result, sem.DeriveRequest) (string, error) {
		return "", &fakeGenError{kind: "unsupported"}
	})
	if out := res.Render(); !strings.Contains(out, "E1432") {
		t.Fatalf("expected E1432:\n%s", out)
	}
}
