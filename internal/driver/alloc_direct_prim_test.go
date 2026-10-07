package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

const directPrimLoopSrc = `fn run(data: Bytes, xs: &Array[Int]) -> Int {
    let mut total = 0
    let mut i: usize = 0
    for i < 2000 {
        total = total + (i.toU8() || 0).toInt() + (xs.get(i % 4) || 0) + (data.get(i % 5) || 0).toInt()
        i = i + 1
    }
    total + (data.len().toInt() || 0) + (xs.len().toInt() || 0)
}

let xs = Array.of(1, 2, 3)
print "total=${run("hello".bytes(), &xs)}"
`

func TestDirectPrimitiveResultsDoNotAllocate(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	m := loadTryConstructorModule(t, directPrimLoopSrc)
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_FAULT_INJECT", "-DKIGUMI_RC_CHECK"}})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT=100")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if code := exitCodeOf(cmd.Run()); code != 0 || out.String() != "total=248448\n" {
		t.Errorf("exit %d, stdout=%q stderr=%q: the loop allocated at least 100 times", code, out.String(), errOut.String())
	}
}
