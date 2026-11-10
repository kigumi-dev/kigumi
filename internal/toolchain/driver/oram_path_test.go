package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// TestPathOramShapesAndData runs one fixture on the VM and natively. The
// fixture checks two things the ORAM contract needs: a storage adapter sees
// the same read/write trace for different secret indices, and every value
// reads back after interleaved writes.
func TestPathOramShapesAndData(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	path := "../../../testdata/run/path_oram.txtar"
	root, wantOut, wantErr, wantExit := loadRunFixture(t, path)

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg"})
	if err != nil {
		t.Fatalf("vm: %v", err)
	}
	if out.String() != wantOut || errOut.String() != wantErr || code != wantExit {
		t.Fatalf("vm: got (%q, %q, %d), want (%q, %q, %d)", out.String(), errOut.String(), code, wantOut, wantErr, wantExit)
	}

	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe)
	cmd.Dir = t.TempDir()
	cmd.Env = os.Environ()
	var nativeOut, nativeErr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &nativeOut, &nativeErr
	nativeCode := 0
	if err := cmd.Run(); err != nil {
		if ee, isExit := err.(*exec.ExitError); isExit {
			nativeCode = ee.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	if nativeOut.String() != wantOut || nativeErr.String() != wantErr || nativeCode != wantExit {
		t.Fatalf("native: got (%q, %q, %d), want (%q, %q, %d)", nativeOut.String(), nativeErr.String(), nativeCode, wantOut, wantErr, wantExit)
	}
}
