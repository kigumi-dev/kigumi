package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func TestSpecializationKeepsAWideRecursionOnTheSharedBody(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	src := `type R = {
    pub a Int
    pub b Int
    pub c Int
    pub d Int
    pub e Int
    pub f Int
}

fn deep[T](x: T, n: Int) -> T {
    if n == 0 { x } else { deep(x, n - 1) }
}

let r = deep(R { a: 1, b: 2, c: 3, d: 4, e: 5, f: 6 }, 60000)
print "${r.f}"
`
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"deepspec\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	load := func() *driver.Module {
		m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	var out, errOut bytes.Buffer
	if code, err := driver.Run(load(), &out, &errOut, []string{"deepspec"}); err != nil || code != 0 || out.String() != "6\n" {
		t.Fatalf("vm: %v exit %d out %q err %q", err, code, out.String(), errOut.String())
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	if ok, err := testBuild(load(), exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	got, err := exec.Command(exe).CombinedOutput()
	if err != nil || string(got) != "6\n" {
		t.Fatalf("native: %v, output %q", err, got)
	}
}
