package driver_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// TestHashConsistency runs testdata/run/hash_consistency.txtar on the VM
// directly and on the native build, so a hash() drift between the two
// engines fails here even when it would not fail the shared-fixture checks.
func TestHashConsistency(t *testing.T) {
	path := "../../../testdata/run/hash_consistency.txtar"
	root, wantOut, _, _ := loadRunFixture(t, path)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}

	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, []string{"main/main.kg"})
	if err != nil || code != 0 {
		t.Fatalf("vm: err %v exit %d\n%s", err, code, errOut.String())
	}
	if out.String() != wantOut {
		t.Errorf("vm stdout mismatch\n--- got ---\n%s--- want ---\n%s", out.String(), wantOut)
	}

	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler for the native build")
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe)
	cmd.Dir = t.TempDir()
	var nativeOut bytes.Buffer
	cmd.Stdout = &nativeOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("native run: %v", err)
	}
	if nativeOut.String() != wantOut {
		t.Errorf("native stdout mismatch\n--- got ---\n%s--- want ---\n%s", nativeOut.String(), wantOut)
	}
}
