package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// fsDevFullSrc is AUD-17's reproduction: stdio buffers the short write and
// the device's ENOSPC surfaces at close.
const fsDevFullSrc = `import fs from std/fs

fn main() {
    let files = host.files()
    let p = match fs.Path.fromString("/dev/full") { Ok(v) -> v, Err(_) -> return }
    match files.create(p) {
        Ok(f) -> {
            let mut f = f
            match f.write("abc".toBytes()) {
                Ok(n) -> print "write=ok ${n}"
                Err(e) -> print "write=err ${e.message()}"
            }
            match f.close() {
                Ok(_) -> print "close=ok"
                Err(e) -> print "close=err ${e.message()}"
            }
        }
        Err(e) -> print "open=err ${e.message()}"
    }
}
`

const fsDevFullWant = "write=ok 3\nclose=err No space left on device\n"

// TestFileWriteAndCloseReportDevFullTheSameOnVMAndNative pins both engines to
// one /dev/full outcome: the buffered write reports success and close reports
// ENOSPC. A program that only checks write must not believe a full device
// accepted its data, and neither engine may swallow the close error.
func TestFileWriteAndCloseReportDevFullTheSameOnVMAndNative(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/dev/full is a Linux device")
	}
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skipf("/dev/full is unavailable: %v", err)
	}
	std, _ := filepath.Abs("../../../std")
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	os.WriteFile(filepath.Join(root, "main/main.kg"), []byte(fsDevFullSrc), 0o644)

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("vm", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg"})
		if err != nil || code != 0 {
			t.Fatalf("err %v exit %d\n%s", err, code, errOut.String())
		}
		if got := out.String(); got != fsDevFullWant {
			t.Errorf("stdout = %q, want %q", got, fsDevFullWant)
		}
	})

	t.Run("build", func(t *testing.T) {
		if testCompiler().CCompiler() == nil {
			t.Skip("no C compiler")
		}
		exe := filepath.Join(t.TempDir(), "prog")
		var buildErr bytes.Buffer
		if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
			t.Fatalf("build: %v\n%s", err, buildErr.String())
		}
		cmd := exec.Command(exe)
		cmd.Dir = t.TempDir()
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v\nstderr: %s", err, errOut.String())
		}
		if got := out.String(); got != fsDevFullWant {
			t.Errorf("native stdout = %q, want %q", got, fsDevFullWant)
		}
	})
}
