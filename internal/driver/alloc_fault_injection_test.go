package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"kigumi/internal/driver"
)

func buildFaultInjectable(t *testing.T, src string) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"allocfault\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_FAULT_INJECT"}})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	return exe
}

// Builds with -DKIGUMI_FAULT_INJECT (rt_core.c's test-only fault-injection hook) and checks Arena.tryCreate reports a refused calloc instead of crashing.
func TestArenaCreateReportsAllocFailureNatively(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := buildFaultInjectable(t, "import alloc from std/alloc\n\nprint \"${alloc.Arena.tryCreate() is Ok(_)}\"\n")
	for _, c := range []struct {
		name string
		fail bool
		want string
	}{
		{"ok", false, "true\n"},
		{"injected failure", true, "false\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(exe)
			cmd.Env = os.Environ()
			if c.fail {
				cmd.Env = append(cmd.Env, "KIGUMI_FAIL_ARENA_CREATE=1")
			}
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v (arena creation failure must not crash the process)", err)
			}
			if string(out) != c.want {
				t.Errorf("got %q, want %q", out, c.want)
			}
		})
	}
}

func TestArenaCreateFallibleReportsAllocFailureNatively(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := buildFaultInjectable(t, "import alloc from std/alloc\n\nmatch alloc.Arena.create() {\n    Ok(_) -> print \"ok\"\n    Err(e) -> print \"err: ${e.message()}\"\n}\n")
	for _, c := range []struct {
		name string
		fail bool
		want string
	}{
		{"ok", false, "ok\n"},
		{"injected failure", true, "err: out of memory\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(exe)
			cmd.Env = os.Environ()
			if c.fail {
				cmd.Env = append(cmd.Env, "KIGUMI_FAIL_ARENA_CREATE=1")
			}
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if string(out) != c.want {
				t.Errorf("got %q, want %q", out, c.want)
			}
		})
	}
}

func TestArenaCreateFallibleReportsAllocFailureVM(t *testing.T) {
	t.Parallel()
	code, out, errOut := vmAtFirstAlloc(t, "import alloc from std/alloc\n\nmatch alloc.Arena.create() {\n    Ok(_) -> print \"ok\"\n    Err(e) -> print \"err: ${e.message()}\"\n}\n")
	if code != 0 || out != "err: out of memory\n" {
		t.Errorf("exit %d stdout=%q stderr=%q", code, out, errOut)
	}
}

const sharedNewFaultSrc = "import alloc from std/alloc\n\nlet s = alloc.Shared.new(41)\nprint \"${s.get() + 1}\"\n"

func TestSharedNewPanicsOnAllocFailureVM(t *testing.T) {
	t.Parallel()
	code, out, errOut := vmAtFirstAlloc(t, sharedNewFaultSrc)
	if code != 2 || out != "" || errOut != "panic: out of memory\n" {
		t.Errorf("exit %d stdout=%q stderr=%q", code, out, errOut)
	}
}

func TestSharedNewPanicsOnAllocFailureNatively(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := buildFaultInjectable(t, sharedNewFaultSrc)
	panicked := false
	for n := 1; n <= 6; n++ {
		cmd := exec.Command(exe)
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		err := cmd.Run()
		if err == nil {
			if out.String() != "42\n" {
				t.Errorf("N=%d: stdout=%q", n, out.String())
			}
			continue
		}
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 2 {
			t.Fatalf("N=%d: %v stderr=%q", n, err, errOut.String())
		}
		if errOut.String() == "panic: out of memory\n" {
			panicked = true
		}
	}
	if !panicked {
		t.Error("no allocation index made Shared.new panic with out of memory")
	}
}
