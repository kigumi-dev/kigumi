package explain_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func TestExplainFnComposesCallbackFacts(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "main"), 0o755)
	file := filepath.Join(dir, "main", "main.kg")
	os.WriteFile(file, []byte(`
import {Box, Shared} from std/alloc

fn boxPanics(b: &Box[Int]) -> Int {
    b.with((x: &Int) => Some(1).expect("present") + x)
}

fn sharedPanics(s: &Shared[Int]) -> Int {
    s.with((x: &Int) => Some(1).expect("present") + x)
}

fn arrayPure(a: &Array[Int]) -> Bool? {
    a.with(0, (x: &Int) => x == 1)
}

fn boxPure(b: &Box[Int]) -> Bool {
    b.with((x: &Int) => x == 1)
}
`), 0o644)
	for _, fn := range []string{"main.boxPanics", "main.sharedPanics"} {
		code, out, errOut := clitest.Execute(t, "--std", std, "explain", "--fn", fn, file)
		if code != 0 || !strings.Contains(out, "may panic") {
			t.Fatalf("explain --fn %s must report the callback's panic: exit %d\n%s%s", fn, code, out, errOut)
		}
	}
	for _, fn := range []string{"main.arrayPure", "main.boxPure"} {
		code, out, errOut := clitest.Execute(t, "--std", std, "explain", "--fn", fn, file)
		if code != 0 || strings.Contains(out, "may panic") || strings.Contains(out, "unverified") {
			t.Fatalf("explain --fn %s must stay free of panic and unverified facts: exit %d\n%s%s", fn, code, out, errOut)
		}
	}
}
