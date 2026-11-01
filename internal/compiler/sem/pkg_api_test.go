package sem_test

import (
	"encoding/json"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

func libAPI(t *testing.T, src string) string {
	t.Helper()
	f := token.NewFile("lib/lib.kg", []byte(src))
	pkgs := append(semfixture.LoadStd(t, "../../../std"), &sem.Package{Path: "lib", Files: []*syntax.Tree{syntax.Parse(f)}})
	res := sem.CheckWith(&sem.Module{Packages: pkgs}, schema.GenerateDerive)
	for i := 1; i < len(res.Files); i++ {
		for _, d := range res.Files[i].Diags {
			if d.Severity == diag.Error {
				t.Fatalf("fixture does not check:\n%s", res.Render())
			}
		}
	}
	api, ok := res.PackageAPIOf("lib")
	if !ok {
		t.Fatal("no lib package")
	}
	b, err := json.Marshal(api)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPackageAPIStability(t *testing.T) {
	t.Parallel()
	const base = `pub type Box[T] = {
    pub value T
}

pub fn Box[T].get(self) -> T {
    self.value
}

pub fn double(x: Int) -> Int {
    helper(x) * 2
}

fn helper(x: Int) -> Int {
    x
}

pub const Limit: Int = 10
`
	want := libAPI(t, base)
	same := map[string]string{
		"private body":       "pub type Box[T] = {\n    pub value T\n}\n\npub fn Box[T].get(self) -> T {\n    self.value\n}\n\npub fn double(x: Int) -> Int {\n    helper(x) * 2\n}\n\nfn helper(x: Int) -> Int {\n    x + 0\n}\n\npub const Limit: Int = 10\n",
		"private addition":   base + "\nfn unused() -> Int {\n    3\n}\n",
		"non-generic body":   "pub type Box[T] = {\n    pub value T\n}\n\npub fn Box[T].get(self) -> T {\n    self.value\n}\n\npub fn double(x: Int) -> Int {\n    let y = helper(x)\n    y * 2\n}\n\nfn helper(x: Int) -> Int {\n    x\n}\n\npub const Limit: Int = 10\n",
		"generic body space": "pub type Box[T] = {\n    pub value T\n}\n\npub fn Box[T].get(self) -> T {\n    // read it\n    self.value\n}\n\npub fn double(x: Int) -> Int {\n    helper(x) * 2\n}\n\nfn helper(x: Int) -> Int {\n    x\n}\n\npub const Limit: Int = 10\n",
	}
	for name, src := range same {
		if got := libAPI(t, src); got != want {
			t.Errorf("%s must not move the API:\n%s\n%s", name, got, want)
		}
	}
	differs := map[string]string{
		"signature":          "pub type Box[T] = {\n    pub value T\n}\n\npub fn Box[T].get(self) -> T {\n    self.value\n}\n\npub fn double(x: Int, y: Int) -> Int {\n    helper(x) * y\n}\n\nfn helper(x: Int) -> Int {\n    x\n}\n\npub const Limit: Int = 10\n",
		"const value":        "pub type Box[T] = {\n    pub value T\n}\n\npub fn Box[T].get(self) -> T {\n    self.value\n}\n\npub fn double(x: Int) -> Int {\n    helper(x) * 2\n}\n\nfn helper(x: Int) -> Int {\n    x\n}\n\npub const Limit: Int = 11\n",
		"generic body":       "pub type Box[T] = {\n    pub value T\n}\n\npub fn Box[T].get(self) -> T {\n    (self.value)\n}\n\npub fn double(x: Int) -> Int {\n    helper(x) * 2\n}\n\nfn helper(x: Int) -> Int {\n    x\n}\n\npub const Limit: Int = 10\n",
		"private field":      "pub type Box[T] = {\n    pub value T\n    count Int\n}\n\npub fn Box[T].get(self) -> T {\n    self.value\n}\n\npub fn double(x: Int) -> Int {\n    helper(x) * 2\n}\n\nfn helper(x: Int) -> Int {\n    x\n}\n\npub const Limit: Int = 10\n",
		"new public item":    base + "\npub fn extra() -> Int {\n    1\n}\n",
		"visibility removed": "pub type Box[T] = {\n    pub value T\n}\n\npub fn Box[T].get(self) -> T {\n    self.value\n}\n\npub fn double(x: Int) -> Int {\n    helper(x) * 2\n}\n\nfn helper(x: Int) -> Int {\n    x\n}\n\nconst Limit: Int = 10\n",
	}
	for name, src := range differs {
		if got := libAPI(t, src); got == want {
			t.Errorf("%s must move the API", name)
		}
	}
}

func TestPackageAPIEffectSummary(t *testing.T) {
	t.Parallel()
	pure := libAPI(t, "pub fn size(x: Int) -> Int {\n    x * 2\n}\n")
	allocating := libAPI(t, "pub fn size(x: Int) -> Int {\n    let a = [x, x]\n    a.len()\n}\n")
	if pure == allocating {
		t.Fatal("a body that starts allocating changes the exported effect summary")
	}
}

func TestPackageAPIIgnoresInstancesMintedByImporters(t *testing.T) {
	t.Parallel()
	const lib = "pub interface Shape {\n    fn area(self) -> Int\n}\n\npub type Square = {\n    pub n Int\n}\n\npub fn Square.area(self) -> Int {\n    self.n\n}\n\npub fn measure[S: Shape](s: S) -> Int {\n    s.area()\n}\n"
	const app = "import lib from lib\n\nfn use() -> Int {\n    lib.measure(lib.Square { n: 1 })\n}\n"
	api := func(withApp bool) string {
		pkgs := append(semfixture.LoadStd(t, "../../../std"), &sem.Package{Path: "lib", Files: []*syntax.Tree{syntax.Parse(token.NewFile("lib/lib.kg", []byte(lib)))}})
		if withApp {
			pkgs = append(pkgs, &sem.Package{Path: "app", Files: []*syntax.Tree{syntax.Parse(token.NewFile("app/app.kg", []byte(app)))}})
		}
		res := sem.CheckWith(&sem.Module{Packages: pkgs}, schema.GenerateDerive)
		if res.HasErrors() {
			t.Fatalf("fixture does not check:\n%s", res.Render())
		}
		a, _ := res.PackageAPIOf("lib")
		b, _ := json.Marshal(a)
		return string(b)
	}
	if alone, used := api(false), api(true); alone != used {
		t.Errorf("an importer instantiating a generic must not move the exporter's API:\n%s\n%s", alone, used)
	}
}
