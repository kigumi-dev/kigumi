package driver_test

import (
	"strings"
	"testing"
)

// lazyAppSteps checks app at each step after a cold check of files, and wants the imports checked lazily without a retry.
func lazyAppSteps(t *testing.T, files map[string]string, want string, apps ...string) {
	t.Helper()
	root, cache := t.TempDir(), t.TempDir()
	writeGraph(t, root, files)
	sameAsWhole(t, root, cache, "cmd/app/main.kg")
	for i, app := range apps {
		writeGraph(t, root, map[string]string{"cmd/app/main.kg": app})
		m, got, _ := unitCheck(t, root, cache, "cmd/app/main.kg")
		whole, _ := wholeCheck(t, root, "cmd/app/main.kg")
		if got != whole || !strings.Contains(got, want) {
			t.Fatalf("step %d: want %q, the unit check must match the whole check\n--- unit\n%s--- whole\n%s", i, want, got, whole)
		}
		st := m.UnitStats()
		if st.Lazy != 1 || st.LazyRetries != 0 || st.SkippedBodies == 0 || st.Checked["xmod/lib"] != 0 {
			t.Fatalf("step %d: one lazy run over app alone: %+v", i, st)
		}
	}
}

func TestUnitLazyImportDropGlue(t *testing.T) {
	t.Parallel()
	lib := "pub type Res = resource {\n    pub n Int\n}\n\npub fn make(n: Int) -> Res {\n    Res { n: n }\n}\n\nfn Res.drop(move self) -> Unit {\n    _ = \"${self.n}\"\n}\n"
	app := "import lib from xmod/lib\n\nnoalloc fn take(r: lib.Res) -> Int {\n    r.n + %s\n}\n\nprint \"${take(lib.make(1))}\"\n"
	files := map[string]string{"mod.kg": scenarioMod, "lib/lib.kg": lib, "cmd/app/main.kg": strings.Replace(strings.Replace(app, "%s", "0", 1), "noalloc ", "", 1)}
	lazyAppSteps(t, files, "dropping `Res` may allocate", strings.Replace(app, "%s", "1", 1), strings.Replace(app, "%s", "2", 1))
}

func TestUnitLazyImportChain(t *testing.T) {
	t.Parallel()
	leaf := "pub fn grow(x: Int) -> Int {\n    _ = \"${x}\"\n    x\n}\n\npub fn calm(x: Int) -> Int {\n    x\n}\n"
	lib := "import leaf from xmod/leaf\n\npub fn size(x: Int) -> Int {\n    leaf.grow(x) + 1\n}\n\npub fn quiet(x: Int) -> Int {\n    leaf.calm(x)\n}\n"
	app := "import lib from xmod/lib\n\nnoalloc fn use(x: Int) -> Int {\n    lib.%s(x)\n}\n\nprint \"${use(2)}\"\n"
	files := map[string]string{"mod.kg": scenarioMod, "leaf/leaf.kg": leaf, "lib/lib.kg": lib, "cmd/app/main.kg": strings.Replace(app, "%s", "quiet", 1)}
	lazyAppSteps(t, files, "error[", strings.Replace(app, "%s", "size", 1))
	lazyAppSteps(t, files, "", strings.Replace(app, "%s", "quiet", 1)+"\n")
}

func TestUnitLazyPrivateCallAcrossPackages(t *testing.T) {
	t.Parallel()
	lib := "pub fn open(x: Int) -> Int {\n    x\n}\n\nfn secret(x: Int) -> Int {\n    _ = \"${x}\"\n    x\n}\n"
	app := "import lib from xmod/lib\n\nprint \"${lib.%s(2)}\"\n"
	files := map[string]string{"mod.kg": scenarioMod, "lib/lib.kg": lib, "cmd/app/main.kg": strings.Replace(app, "%s", "open", 1)}
	lazyAppSteps(t, files, "error[", strings.Replace(app, "%s", "secret", 1))
}
