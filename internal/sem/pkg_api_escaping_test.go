package sem_test

import (
	"testing"

	"kigumi/internal/diag"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

func TestPackageAPITracksEscapingTypeParam(t *testing.T) {
	t.Parallel()
	const plain = "pub fn run[F: Fn[fn() -> Unit]](callback: F) -> Unit {\n    callback()\n}\n"
	const escaping = "pub fn run[F: Fn[fn() -> Unit]](callback: escaping F) -> Unit {\n    callback()\n}\n"
	if libAPI(t, plain) == libAPI(t, escaping) {
		t.Error("granting `escaping F` must move the exported API")
	}
	const viaPlain = "pub fn reg[F: Fn[fn() -> Unit]](r: fn(F) -> Unit, f: F) -> Unit {\n    r(f)\n}\n"
	const viaEscaping = "pub fn reg[F: Fn[fn() -> Unit]](r: fn(escaping F) -> Unit, f: escaping F) -> Unit {\n    r(f)\n}\n"
	if libAPI(t, viaPlain) == libAPI(t, viaEscaping) {
		t.Error("`escaping F` inside a fn type parameter must move the exported API")
	}
}

func TestParamTypeStringKeepsEscaping(t *testing.T) {
	t.Parallel()
	const src = "pub fn keep[F: Fn[fn() -> Unit]](callback: escaping F, other: F) -> Unit {}\npub fn keepV[F: Fn[fn() -> Unit]](cbs: ...escaping F) -> Unit {}\npub fn plainV[F: Fn[fn() -> Unit]](cbs: ...F) -> Unit {}\n"
	pkgs := append(testkit.LoadStd(t, "../../std"), &sem.Package{Path: "lib", Files: []*syntax.Tree{syntax.Parse(token.NewFile("lib/lib.kg", []byte(src)))}})
	res := sem.CheckWith(&sem.Module{Packages: pkgs}, schema.GenerateDerive)
	for _, d := range res.Files[len(res.Files)-1].Diags {
		if d.Severity == diag.Error {
			t.Fatalf("fixture does not check:\n%s", res.Render())
		}
	}
	for i := range res.Entities {
		id := sem.EntityID(i)
		e := res.Entity(id)
		if e.Kind == sem.EntFn && (e.Name == "keepV" || e.Name == "plainV") {
			want := map[string]string{"keepV": "...escaping F", "plainV": "Array[F]"}[e.Name]
			if got := res.ParamTypeString(res.Fn(id).Params[0]); got != want {
				t.Errorf("%s variadic printed as %q, want %q", e.Name, got, want)
			}
			continue
		}
		if e.Kind != sem.EntFn || e.Name != "keep" {
			continue
		}
		params := res.Fn(id).Params
		if got := res.ParamTypeString(params[0]); got != "escaping F" {
			t.Errorf("escaping parameter printed as %q", got)
		}
		if got := res.ParamTypeString(params[1]); got != "F" {
			t.Errorf("plain parameter printed as %q", got)
		}
	}
}
