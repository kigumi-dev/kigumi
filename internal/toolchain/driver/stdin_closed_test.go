//go:build unix

package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const stdinClosedSrc = `let s = host.stdin().readAll()?
print "got <${s}> ${s.len()}"
`

func writeStdinModule(t *testing.T) string {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"stdinclosed\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(stdinClosedSrc), 0o644)
	return root
}

func runWithStdin(t *testing.T, redirect string, argv ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command("/bin/sh", append([]string{"-c", `exec "$@" ` + redirect, "sh"}, argv...)...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if x, ok := err.(*exec.ExitError); ok {
		return out.String(), errOut.String(), x.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return out.String(), errOut.String(), 0
}

// The Go runtime reopens a closed fd 0 as /dev/null O_RDWR before main, which is
// indistinguishable from `0<>/dev/null`, so only the native binary sees the closure.
func TestStdinClosedAndDevNull(t *testing.T) {
	if os.Getenv("KIGUMI_STDIN_ROLE") == "vm" {
		m, err := driver.LoadModule(os.Getenv("KIGUMI_STDIN_ROOT"), driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
		if err != nil {
			t.Fatal(err)
		}
		code, err := driver.Run(m, os.Stdout, os.Stderr, nil)
		if err != nil {
			t.Fatal(err)
		}
		os.Exit(code)
	}
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := writeStdinModule(t)
	exe := filepath.Join(root, "prog")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
	if err != nil {
		t.Fatal(err)
	}
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	t.Setenv("KIGUMI_STDIN_ROLE", "vm")
	t.Setenv("KIGUMI_STDIN_ROOT", root)
	vmArgs := []string{os.Args[0], "-test.run=^TestStdinClosedAndDevNull$", "-test.count=1"}
	for _, tc := range []struct {
		name, redirect string
		argv           []string
		stdout, stderr string
		code           int
	}{
		{"native devnull", "0<>/dev/null", []string{exe}, "got <> 0\n", "", 0},
		{"vm devnull", "0<>/dev/null", vmArgs, "got <> 0\n", "", 0},
		{"native closed", "0<&-", []string{exe}, "", "error: Bad file descriptor", 1},
		{"vm closed", "0<&-", vmArgs, "got <> 0\n", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runWithStdin(t, tc.redirect, tc.argv...)
			if stdout != tc.stdout || !strings.Contains(stderr, tc.stderr) || code != tc.code {
				t.Fatalf("stdout=%q stderr=%q code=%d", stdout, stderr, code)
			}
		})
	}
}
