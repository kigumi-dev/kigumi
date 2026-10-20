package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

// Builds with -DKIGUMI_FAULT_INJECT (rt_core.c's test-only fault-injection hook) and checks Arena.tryCreate reports a refused calloc instead of crashing.
func TestArenaCreateReportsAllocFailureNatively(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"allocfault\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("import alloc from std/alloc\n\nprint \"${alloc.Arena.tryCreate() is Ok(_)}\"\n"), 0o644)
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
