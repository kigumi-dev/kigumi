package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// hostShellOpsSrc exercises the REPL's host working-directory, environment
// and foreground-pipeline operations. Both executors must produce the same
// bytes: the tests below compare them.
const hostShellOpsSrc = `import os from std/os
import shell from std/shell

fn envText(name: String) -> String {
    match host.env(name) {
        Some(v) -> v
        None -> "<none>"
    }
}

fn lastStatus(result: shell.PipelineResult) -> Int {
    let mut last = 0
    for s in result.statuses {
        last = s
    }
    last
}

let sh = host.shell()
print "cwd0=${host.cwd()}"
host.chdir("sub")?
print "cwd1=${host.cwd()}"
host.setEnv("KIGUMI_HOST_OPS", "v1")?
print "env1=${envText("KIGUMI_HOST_OPS")}"
host.unsetEnv("KIGUMI_HOST_OPS")?
print "env2=${envText("KIGUMI_HOST_OPS")}"
match $"printf ran" |> shell.runHere(sh) {
    Ok(r) -> print "status=${lastStatus(r)}"
    Err(e) -> print "here-failed: ${e.message()}"
}
match $"sh -c false" |> shell.runHere(sh) {
    Ok(r) -> print "status1=${lastStatus(r)}"
    Err(e) -> print "here-failed: ${e.message()}"
}
match $"/nonexistent-kigumi-host-ops" |> shell.runHere(sh) {
    Ok(_) -> print "start-ok"
    Err(_) -> print "start-failed"
}
match $"printf x > /nonexistent-kigumi-host-ops-dir/out" |> shell.runHere(sh) {
    Ok(_) -> print "redirect-ok"
    Err(_) -> print "redirect-failed"
}
match $"printf a | grep a | cat" |> shell.runHere(sh) {
    Ok(r) -> print "chain=${r.statuses.len()}"
    Err(e) -> print "chain-failed: ${e.message()}"
}
match host.chdir("does-not-exist") {
    Ok(_) -> print "bad-chdir"
    Err(_) -> print "chdir-failed"
}
print "cwd2=${host.cwd()}"
`

func hostShellOpsModule(t *testing.T) (*driver.Module, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"hostshellops\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(hostShellOpsSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	return m, root
}

func hostShellOpsWant(root string) string {
	sub := filepath.Join(root, "sub")
	// printf(1) writes "ran" with no newline, so the next print follows it.
	return fmt.Sprintf("cwd0=%s\ncwd1=%s\nenv1=v1\nenv2=<none>\nranstatus=0\nstatus1=1\nstart-failed\nredirect-failed\na\nchain=3\nchdir-failed\ncwd2=%s\n", root, sub, sub)
}

// TestHostChdirCwdEnvVM covers the VM accelerators for chdir, cwd, setEnv and
// unsetEnv, and runHere's statuses, inherited output and startup rejection.
func TestHostChdirCwdEnvVM(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux cwd semantics (the REPL's v1 target)")
	}
	m, root := hostShellOpsModule(t)
	oldWD, _ := os.Getwd()
	oldEnv, hadEnv := os.LookupEnv("KIGUMI_HOST_OPS")
	t.Cleanup(func() {
		os.Chdir(oldWD)
		if hadEnv {
			os.Setenv("KIGUMI_HOST_OPS", oldEnv)
		} else {
			os.Unsetenv("KIGUMI_HOST_OPS")
		}
	})
	os.Unsetenv("KIGUMI_HOST_OPS")

	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: root}, &out, &errOut, []string{"main.kg"})
	if err != nil || code != 0 {
		t.Fatalf("vm: err %v exit %d\nstderr: %s", err, code, errOut.String())
	}
	if got, want := out.String(), hostShellOpsWant(root); got != want {
		t.Errorf("vm stdout mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

// TestHostChdirCwdEnvNative pins the same program on the native executor so
// the two paths cannot drift.
func TestHostChdirCwdEnvNative(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux cwd semantics (the REPL's v1 target)")
	}
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	m, root := hostShellOpsModule(t)
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe)
	cmd.Dir = root
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, errOut.String())
	}
	if got, want := out.String(), hostShellOpsWant(root); got != want {
		t.Errorf("native stdout mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
