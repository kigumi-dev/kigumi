package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func compileFor(t *testing.T, target driver.Target, src string) (out string, ok bool) {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"prov\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_, ok, err = driver.Compile(m, &buf)
	if err != nil {
		t.Fatal(err)
	}
	return buf.String(), ok
}

func mustTarget(t *testing.T, triple, sys string) driver.Target {
	t.Helper()
	tgt, err := driver.ParseTarget(driver.CompilerOptions{}, triple, sys)
	if err != nil {
		t.Fatal(err)
	}
	return tgt
}

const hostArgsSrc = "let args = host.args()\nprint \"${args.len()}\"\n"

func TestCapabilityProvisionRejectsOnEveryTarget(t *testing.T) {
	t.Parallel()
	const pool = "import task from std/task\n\nasync fn leaf(n: Int) -> Int {\n    n\n}\n\nlet pool = task.threadPoolExecutor(2)\nprint \"${pool.runToCompletion(leaf(1))}\"\n"
	cases := []struct {
		name, src string
		target    driver.Target
		want      []string
	}{
		{"sys none, no strict profile", hostArgsSrc, mustTarget(t, "", "none"), []string{"E1370", "HOST_ARGS", "host.args", "--sys none", "main"}},
		{"wasi sockets", "let c = host.net().http()\nprint \"x\"\n", mustTarget(t, "wasm32-wasi", ""), []string{"E1370", "host.net", "wasi has no sockets"}},
		{"wasi worker threads", pool, mustTarget(t, "wasm32-wasi", ""), []string{"E1370", "TASK_THREAD_POOL_EXECUTOR", "task.threads", "wasi has no worker threads"}},
		{"windows signals", "let s = host.signals()\nprint \"x\"\n", mustTarget(t, "x86_64-windows-gnu", ""), []string{"E1370", "host.signals", "windows has no POSIX signals"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			out, ok := compileFor(t, c.target, c.src)
			if ok {
				t.Fatalf("expected the build to be rejected:\n%s", out)
			}
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("diagnostic lacks %q:\n%s", w, out)
				}
			}
		})
	}
}

func TestCapabilityProvisionAcceptsWhatTheTargetProvides(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, src string
		target    driver.Target
	}{
		{"hosted host.args", hostArgsSrc, driver.HostTarget()},
		{"wasi host.args", hostArgsSrc, mustTarget(t, "wasm32-wasi", "")},
		{"hosted signals", "let s = host.signals()\nprint \"x\"\n", driver.HostTarget()},
		{"hosted net", "let c = host.net().http()\nprint \"x\"\n", driver.HostTarget()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if out, ok := compileFor(t, c.target, c.src); !ok || strings.Contains(out, "E1370") {
				t.Fatalf("expected the build to be accepted, ok=%v:\n%s", ok, out)
			}
		})
	}
}
