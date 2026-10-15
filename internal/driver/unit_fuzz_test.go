package driver_test

import (
	"fmt"
	"testing"

	"kigumi/internal/driver"
)

func richFiles() map[string]string {
	return map[string]string{
		"mod.kg": "Module {\n    name: \"rich\"\n    kigumi: \"0.1\"\n}\n",
		"lib/lib.kg": `pub context token: String

@provides(token)
pub type Bundle = {
    pub token String
}

pub fn Bundle.wrap(t: String) -> Bundle {
    Bundle { token: t }
}

pub interface Shape {
    fn area(self) -> Int
}

pub type Square = {
    pub n Int
}

pub fn Square.area(self) -> Int {
    self.n * self.n
}

pub fn measure[S: Shape](s: S) -> Int {
    s.area()
}

pub fn apply(f: fn(Int) -> Int, x: Int) -> Int {
    f(x)
}

pub fn twice[T](x: T, f: fn(T) -> T) -> T {
    f(f(x))
}

pub noalloc fn clamp(x: Int) -> Int {
    if x < 0 { 0 } else { x }
}

pub fn greet(name: String) -> String {
    "hi ${name}"
}

pub fn needsToken() -> String {
    "none"
}

pub type Shape2 = Circle(r: Int) | Dot

pub type Holder['a] = {
    pub mut r &'a mut Shape2
}

pub fn touch(s: &mut Shape2, k: Int) -> Unit {
    let n = k
}

pub interface Maker {
    noalloc fn make(self, i: Int) -> Int
}

pub type Pm = {
    pub k Int
}

pub noalloc fn Pm.make(self, i: Int) -> Int {
    i + self.k
}

pub fn syncRoot() -> Int {
    1
}
`,
		"mid/mid.kg": `import lib from rich/lib

pub fn bump(x: Int) -> Int {
    lib.clamp(x) + lib.apply((n: Int) => n + 1, x)
}

pub fn label(n: Int) -> String {
    lib.greet("${n}")
}

pub noalloc fn safe(x: Int) -> Int {
    lib.clamp(x)
}

pub async fn later(x: Int) -> Int {
    lib.clamp(x)
}

pub fn area(n: Int) -> Int {
    lib.measure(lib.Square { n: n })
}

pub fn relay['a](h: lib.Holder['a]) -> Unit {
    lib.touch(h.r, 1)
}

pub fn useMaker(m: lib.Maker) -> Int {
    m.make(1)
}

pub async fn bridged() -> Int {
    lib.syncRoot()
}
`,
		"cmd/app/main.kg": `import lib from rich/lib
import mid from rich/mid

noalloc fn quick(x: Int) -> Int {
    mid.safe(x)
}

print "${mid.bump(2)} ${mid.label(3)} ${quick(4)} ${lib.twice(1, (n: Int) => n * 2)} ${mid.area(2)} ${lib.needsToken()} ${mid.useMaker(lib.Pm { k: 1 })}"
`,
	}
}

func TestUnitRichModuleIsClean(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeGraph(t, root, richFiles())
	if got, hasErr := wholeCheck(t, root, "cmd/app/main.kg"); got != "" || hasErr {
		t.Fatalf("the fuzz base must check clean:\n%s", got)
	}
}

func TestUnitFuzzRichModule(t *testing.T) {
	var tot sweepTotals
	for seed := 0; seed < unitFuzzSeeds; seed++ {
		fx := sweepFixture{name: fmt.Sprintf("rich-%d", seed), files: richFiles(), entry: "cmd/app/main.kg", keepClean: true}
		t.Run(fx.name, func(t *testing.T) { sweepFixtureRun(t, fx, 6, &tot) })
	}
	t.Logf("semantic runs=%d replayed packages=%d fallbacks=%d lazy runs=%d lazy retries=%d", tot.runs, tot.replays, tot.fallbacks, tot.lazy, tot.lazyRetries)
}

const unitFuzzSeeds = 12

func TestUnitArtifactsMatchWholeCheck(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeGraph(t, root, richFiles())
	digests := func(units bool) map[string]string {
		m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: stdRoot(t), Entry: "cmd/app/main.kg", PackageCache: t.TempDir(), DeferParse: true, UnitCheck: units})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := driver.CheckUnits(m); err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, s := range m.PackageStatuses() {
			out[s.Path] = s.Digest
		}
		return out
	}
	whole, units := digests(false), digests(true)
	if len(whole) != 3 {
		t.Fatalf("want 3 packages, got %v", whole)
	}
	for p, d := range whole {
		if units[p] != d {
			t.Errorf("%s: the artifact checked as a unit differs from the whole check's", p)
		}
	}
}
