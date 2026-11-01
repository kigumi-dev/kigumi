package check_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/cli/clitest"
)

func TestCheckReplaysUnchangedModuleAndNoticesEdits(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	root := t.TempDir()
	t.Setenv("KIGUMI_PKG_CACHE", t.TempDir())
	write := func(rel, body string) {
		full := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Hour)
		os.Chtimes(full, old, old)
	}
	write("mod.kg", "Module {\n    name: \"settle\"\n    kigumi: \"0.1\"\n}\n")
	write("lib/lib.kg", "pub fn one() -> Int {\n    1\n}\n")
	write("cmd/app/main.kg", "import lib from settle/lib\n\nprint \"${lib.one()}\"\n")
	code, _, first := clitest.Execute(t, "--std", std, "check", root)
	if code != 0 {
		t.Fatalf("first check: exit %d\n%s", code, first)
	}
	code, _, second := clitest.Execute(t, "--std", std, "check", root)
	if code != 0 || second != first {
		t.Fatalf("an unchanged module replays the same outcome: exit %d, %q vs %q", code, second, first)
	}
	write("lib/lib.kg", "pub fn one() -> String {\n    1\n}\n")
	if code, _, errOut := clitest.Execute(t, "--std", std, "check", root); code != 1 || errOut == "" {
		t.Fatalf("an edit must not be answered from the cache: exit %d, stderr %q", code, errOut)
	}
	write("lib/lib.kg", "pub fn one() -> Int {\n    1\n}\n")
	if code, _, errOut := clitest.Execute(t, "--std", std, "check", root); code != 0 {
		t.Fatalf("reverting settles again: exit %d\n%s", code, errOut)
	}
}

func warnOnlyModule() map[string]string {
	return map[string]string{
		"mod.kg":          "Module {\n    name: \"settle\"\n    kigumi: \"0.1\"\n}\n",
		"cmd/app/main.kg": "import map from std/map\n\nprint \"hi\"\n",
	}
}

func TestSettledCheckReplaysTheColorOfTheRequest(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	for _, order := range [][2]string{{"always", "never"}, {"never", "always"}} {
		root := t.TempDir()
		t.Setenv("KIGUMI_PKG_CACHE", t.TempDir())
		clitest.WriteFiles(t, root, warnOnlyModule(), time.Now().Add(-time.Hour))
		outputs := map[string]string{}
		for _, mode := range []string{order[0], order[1], order[0], order[1]} {
			code, _, errOut := clitest.Execute(t, "--std", std, "--color", mode, "check", root)
			if code != 0 || errOut == "" {
				t.Fatalf("--color %s: exit %d, stderr %q", mode, code, errOut)
			}
			if prev, ok := outputs[mode]; ok && prev != errOut {
				t.Errorf("--color %s changed between runs:\n%q\n%q", mode, prev, errOut)
			}
			outputs[mode] = errOut
			if colored := strings.Contains(errOut, "\x1b["); colored != (mode == "always") {
				t.Errorf("--color %s (after %s first): colored=%v in %q", mode, order[0], colored, errOut)
			}
		}
	}
}

func TestCheckNoticesSameSizeEditWithRestoredMtime(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	root := t.TempDir()
	t.Setenv("KIGUMI_PKG_CACHE", t.TempDir())
	stamp := time.Now().Add(-time.Hour)
	files := map[string]string{
		"mod.kg":          "Module {\n    name: \"settle\"\n    kigumi: \"0.1\"\n}\n",
		"lib/lib.kg":      "pub fn one() -> Int {\n    1\n}\n",
		"cmd/app/main.kg": "import lib from settle/lib\n\nprint \"${lib.one()}\"\n",
	}
	clitest.WriteFiles(t, root, files, stamp)
	for range 2 {
		if code, _, errOut := clitest.Execute(t, "--std", std, "check", root); code != 0 {
			t.Fatalf("clean module: exit %d\n%s", code, errOut)
		}
	}
	clitest.WriteFiles(t, root, map[string]string{"lib/lib.kg": "pub fn one() -> Ini {\n    1\n}\n"}, stamp)
	if code, _, errOut := clitest.Execute(t, "--std", std, "check", root); code != 1 || errOut == "" {
		t.Fatalf("a same-size edit with the mtime put back must be seen: exit %d, stderr %q", code, errOut)
	}
}
