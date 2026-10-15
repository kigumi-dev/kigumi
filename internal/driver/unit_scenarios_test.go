package driver_test

import (
	"strings"
	"testing"
)

const scenarioMod = "Module {\n    name: \"xmod\"\n    kigumi: \"0.1\"\n}\n"

type scenarioStep struct {
	lib  string
	want string
	// replayed names the packages that must not be checked again at this step.
	replayed []string
}

func runScenario(t *testing.T, app string, steps []scenarioStep) {
	t.Helper()
	root, cache := t.TempDir(), t.TempDir()
	files := map[string]string{"mod.kg": scenarioMod, "cmd/app/main.kg": app}
	for i, s := range steps {
		files["lib/lib.kg"] = s.lib
		writeGraph(t, root, files)
		m, got, _ := unitCheck(t, root, cache, "cmd/app/main.kg")
		whole, _ := wholeCheck(t, root, "cmd/app/main.kg")
		if got != whole {
			t.Fatalf("step %d: unit check differs from the whole check\n--- unit\n%s--- whole\n%s", i, got, whole)
		}
		if s.want == "" && got != "" || !strings.Contains(got, s.want) {
			t.Fatalf("step %d: want %q in\n%s", i, s.want, got)
		}
		for _, p := range s.replayed {
			if m.UnitStats().Checked["xmod/"+p] != 0 {
				t.Fatalf("step %d: %s must be replayed: %+v", i, p, m.UnitStats())
			}
		}
	}
}

func TestUnitCrossPackageNoallocFlips(t *testing.T) {
	t.Parallel()
	app := "import lib from xmod/lib\n\nnoalloc fn use(x: Int) -> Int {\n    lib.size(x)\n}\n\nprint \"${use(2)}\"\n"
	pure := "pub fn size(x: Int) -> Int {\n    x * 2\n}\n"
	pureToo := "pub fn size(x: Int) -> Int {\n    x * 2 + 0\n}\n"
	allocating := "pub fn size(x: Int) -> Int {\n    let s = \"${x}\"\n    s.len()\n}\n"
	runScenario(t, app, []scenarioStep{
		{lib: pure},
		{lib: pureToo, replayed: []string{"cmd/app"}},
		{lib: allocating, want: "error["},
		{lib: pure},
		{lib: allocating, want: "error["},
	})
}

func TestUnitCrossPackageBlockingWarningFlips(t *testing.T) {
	t.Parallel()
	app := "import lib from xmod/lib\n\nasync fn readIt() -> String! {\n    lib.load(host.files())?\n}\n"
	quiet := "import fs from std/fs\n\npub fn load(files: fs.Files) -> String! {\n    \"x\"\n}\n"
	blocking := "import fs from std/fs\n\npub fn load(files: fs.Files) -> String! {\n    files.read(fs.Path.fromString(\"x\")?)?\n}\n"
	runScenario(t, app, []scenarioStep{
		{lib: quiet},
		{lib: blocking, want: "warning[W1320]"},
		{lib: quiet},
	})
}

func TestUnitCrossPackageContextRequirementFlips(t *testing.T) {
	t.Parallel()
	app := "import lib from xmod/lib\n\nfn run() -> String {\n    lib.needsToken()\n}\n\nprint run()\n"
	head := "pub context token: String\n\n"
	free := head + "pub fn needsToken() -> String {\n    \"none\"\n}\n"
	needy := head + "pub fn needsToken() -> String {\n    token\n}\n"
	runScenario(t, app, []scenarioStep{
		{lib: free},
		{lib: needy, want: "error["},
		{lib: free},
	})
}

func TestUnitCrossPackageEffectFlowsThroughMiddle(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	files := map[string]string{
		"mod.kg":          scenarioMod,
		"mid/mid.kg":      "import lib from xmod/lib\n\npub fn size(x: Int) -> Int {\n    lib.size(x) + 1\n}\n",
		"cmd/app/main.kg": "import mid from xmod/mid\n\nnoalloc fn use(x: Int) -> Int {\n    mid.size(x)\n}\n\nprint \"${use(2)}\"\n",
	}
	libs := []struct {
		src  string
		want string
	}{
		{"pub fn size(x: Int) -> Int {\n    x * 2\n}\n", ""},
		{"pub fn size(x: Int) -> Int {\n    let s = \"${x}\"\n    s.len()\n}\n", "error["},
		{"pub fn size(x: Int) -> Int {\n    x * 3\n}\n", ""},
	}
	for i, l := range libs {
		files["lib/lib.kg"] = l.src
		writeGraph(t, root, files)
		_, got, _ := unitCheck(t, root, cache, "cmd/app/main.kg")
		whole, _ := wholeCheck(t, root, "cmd/app/main.kg")
		if got != whole || (l.want != "" && !strings.Contains(got, l.want)) || (l.want == "" && got != "") {
			t.Fatalf("step %d: unit %q, whole %q", i, got, whole)
		}
	}
}

func TestUnitCrossPackageRootBridgeFlips(t *testing.T) {
	t.Parallel()
	app := "import lib from xmod/lib\n\nasync fn bad() -> Int {\n    lib.syncRoot()\n}\n"
	calm := "pub fn syncRoot() -> Int {\n    1\n}\n"
	driving := "import task from std/task\n\nasync fn work() -> Int {\n    0\n}\n\npub fn syncRoot() -> Int {\n    let ex = task.currentThreadExecutor()\n    ex.runToCompletion(work())\n}\n"
	runScenario(t, app, []scenarioStep{
		{lib: calm},
		{lib: driving, want: "error[E1291]"},
		{lib: calm},
		{lib: driving, want: "error[E1291]"},
	})
}

func TestUnitCrossPackageConformanceFlips(t *testing.T) {
	t.Parallel()
	app := "import lib from xmod/lib\n\nfn use(m: lib.Maker) -> Int {\n    m.make(1)\n}\n\nprint \"${use(lib.Pm { k: 1 })}\"\n"
	head := "pub interface Maker {\n    noalloc fn make(self, i: Int) -> Int\n}\n\npub type Pm = {\n    pub k Int\n}\n\n"
	calm := head + "pub noalloc fn Pm.make(self, i: Int) -> Int {\n    i + self.k\n}\n"
	allocating := head + "pub noalloc fn Pm.make(self, i: Int) -> Int {\n    let s = \"${i}\"\n    s.len() + self.k\n}\n"
	runScenario(t, app, []scenarioStep{
		{lib: calm},
		{lib: allocating, want: "error["},
		{lib: calm},
	})
}

func TestUnitCrossPackageLentCellFlips(t *testing.T) {
	t.Parallel()
	app := "import lib from xmod/lib\n\nfn go['a](h: lib.Holder['a]) -> Unit {\n    lib.touch(h.r, 1)\n}\n\nprint \"ok\"\n"
	head := "pub type Shape = Circle(r: Int) | Dot\n\npub fn Shape.area(self) -> Int {\n    match self {\n        Circle(r) -> r\n        Dot -> 0\n    }\n}\n\npub type Holder['a] = {\n    pub mut r &'a mut Shape\n}\n\n"
	reads := head + "pub fn touch(s: &mut Shape, k: Int) -> Unit {\n    let n = s.area() + k\n}\n"
	replaces := head + "pub fn touch(s: &mut Shape, k: Int) -> Unit {\n    s = Circle(k)\n}\n"
	runScenario(t, app, []scenarioStep{
		{lib: reads},
		{lib: replaces, want: "error[E1503]"},
		{lib: reads},
	})
}

func TestUnitCrossPackageFieldFnWitnessFlips(t *testing.T) {
	t.Parallel()
	app := "import lib from xmod/lib\n\nfn plainView(x: Int) -> Int {\n    x * 2\n}\n\npure fn use(x: Int) -> Int {\n    lib.run(lib.App { view: plainView }, x)\n}\n\nprint \"${use(2)}\"\n"
	head := "pub type App = {\n    pub view pure? fn(Int) -> Int\n}\n\n"
	ignores := head + "pub fn run(a: App, x: Int) -> Int {\n    x + 1\n}\n"
	calls := head + "pub fn run(a: App, x: Int) -> Int {\n    a.view(x)\n}\n"
	runScenario(t, app, []scenarioStep{
		{lib: ignores},
		{lib: calls, want: "cmd/app/main.kg"},
		{lib: ignores},
		{lib: calls, want: "cmd/app/main.kg"},
	})
}

func TestUnitCrossPackageDirectAggregateFlips(t *testing.T) {
	t.Parallel()
	app := "import lib from xmod/lib\n\nnoalloc fn use(x: Int) -> Int {\n    let p = lib.make(x)\n    p.a + lib.sum(p)\n}\n\nprint \"${use(2)}\"\n"
	head := "pub type Pair = {\n    pub a Int\n    pub b Int\n}\n\n"
	plain := head + "pub fn make(x: Int) -> Pair {\n    Pair { a: x, b: 1 }\n}\n\npub fn sum(p: Pair) -> Int {\n    p.a + p.b\n}\n"
	same := head + "pub fn make(x: Int) -> Pair {\n    Pair { a: x, b: 2 }\n}\n\npub fn sum(p: Pair) -> Int {\n    p.a + p.b + 0\n}\n"
	allocating := head + "pub fn make(x: Int) -> Pair {\n    let s = \"${x}\"\n    Pair { a: x, b: 1 }\n}\n\npub fn sum(p: Pair) -> Int {\n    p.a + p.b\n}\n"
	runScenario(t, app, []scenarioStep{
		{lib: plain},
		{lib: same, replayed: []string{"cmd/app"}},
		{lib: allocating, want: "cmd/app/main.kg"},
		{lib: plain},
	})
}
