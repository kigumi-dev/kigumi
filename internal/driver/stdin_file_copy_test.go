package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

const stdinToFileCopySrc = `import fs from std/fs
import io from std/io

let files = host.files()
let path = fs.Path.fromString("copied.txt")?
let mut f = files.create(path)?
let mut stdin = host.stdin()
let n = io.copy(stdin, f)?
let content = files.readBytes(path)?
print "copied=${n} content=${String.fromBytes(content) || "?"}"
files.remove(path)?
`

// TestStdinToFileIoCopy proves os.Stdin and fs.File satisfy std/io's
// Reader/Writer: io.copy streams stdin into a file. testdata/run fixtures
// can't inject Stdin without blocking, so this sets it directly like TestStdinReadAllReportsRealError.
func TestStdinToFileIoCopy(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	os.WriteFile(filepath.Join(root, "main/main.kg"), []byte(stdinToFileCopySrc), 0o644)

	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("hello from stdin")
	want := fmt.Sprintf("copied=%d content=%s", len(data), data)

	pipeWith := func(t *testing.T) *os.File {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
		w.Close()
		return r
	}

	t.Run("vm", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var out, errOut bytes.Buffer
		code, err := driver.RunWith(m, driver.RunOptions{Stdin: pipeWith(t)}, &out, &errOut, []string{"main/main.kg"})
		if err != nil || code != 0 {
			t.Fatalf("err %v exit %d\n%s", err, code, errOut.String())
		}
		if strings.TrimSpace(out.String()) != want {
			t.Errorf("stdout = %q, want %q", out.String(), want)
		}
	})

	t.Run("build", func(t *testing.T) {
		if driver.CCompiler() == nil {
			t.Skip("no C compiler")
		}
		exe := filepath.Join(t.TempDir(), "prog")
		var buildErr bytes.Buffer
		if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
			t.Fatalf("build: %v\n%s", err, buildErr.String())
		}
		cmd := exec.Command(exe)
		cmd.Dir = t.TempDir()
		cmd.Stdin = pipeWith(t)
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v\nstderr: %s", err, errOut.String())
		}
		if strings.TrimSpace(out.String()) != want {
			t.Errorf("build stdout = %q, want %q", out.String(), want)
		}
	})
}
