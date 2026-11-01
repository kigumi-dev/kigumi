package check_test

import (
	"path/filepath"
	"testing"
	"time"

	"kigumi/internal/cli/clitest"
)

// `kigumi check` prints the same with and without package units across a private edit, an edit that breaks a dependant, and its revert.
func TestCheckUnitsPrintTheWholeCheckOutput(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	const mod = "Module {\n    name: \"units\"\n    kigumi: \"0.1\"\n}\n"
	app := "import mid from units/mid\n\nnoalloc fn use(x: Int) -> Int {\n    mid.size(x)\n}\n\nprint \"${use(2)}\"\n"
	mid := "import lib from units/lib\n\npub fn size(x: Int) -> Int {\n    lib.size(x) + 1\n}\n"
	libs := []string{
		"pub fn size(x: Int) -> Int {\n    x * 2\n}\n",
		"pub fn size(x: Int) -> Int {\n    x * 2 + 0\n}\n",
		"pub fn size(x: Int) -> Int {\n    let s = \"${x}\"\n    s.len()\n}\n",
		"pub fn size(x: Int) -> Int {\n    x * 2\n}\n",
	}
	run := func(units string) []string {
		t.Setenv("KIGUMI_PKG_CACHE", t.TempDir())
		t.Setenv("KIGUMI_PKG_UNITS", units)
		root := t.TempDir()
		var out []string
		for _, lib := range libs {
			writeSettleModule(t, root, map[string]string{"mod.kg": mod, "lib/lib.kg": lib, "mid/mid.kg": mid, "cmd/app/main.kg": app}, time.Now().Add(-time.Hour))
			code, _, errOut := clitest.Execute(t, "--std", std, "check", root)
			out = append(out, errOut+string(rune('0'+code)))
		}
		return out
	}
	on, off := run(""), run("off")
	for i := range libs {
		if on[i] != off[i] {
			t.Errorf("step %d differs with units on:\n%s\n--- off\n%s", i, on[i], off[i])
		}
	}
	if on[2] == on[0] {
		t.Errorf("the allocating edit must break the noalloc caller: %q", on[2])
	}
}
