package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func checkFor(t *testing.T, target driver.Target, src string) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"provcheck\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasErrors() {
		return ""
	}
	return res.Render()
}

func TestCheckReportsCapabilityNotProvided(t *testing.T) {
	t.Parallel()
	const shellSrc = "let sh = host.shell()\nprint \"x\"\n"
	cases := []struct {
		name, src string
		target    driver.Target
		want      []string
	}{
		{"sys none shell", shellSrc, mustTarget(t, "", "none"), []string{"E1370", "host.shell", "no operating system services"}},
		{"wasi shell", shellSrc, mustTarget(t, "wasm32-wasi", ""), []string{"E1370", "host.shell", "wasi has no process spawning"}},
		{"windows signals", "let s = host.signals()\nprint \"x\"\n", mustTarget(t, "x86_64-windows-gnu", ""), []string{"E1370", "host.signals"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			out := checkFor(t, c.target, c.src)
			if out == "" {
				t.Fatal("expected check to report E1370")
			}
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("diagnostic lacks %q:\n%s", w, out)
				}
			}
			if build, _ := compileFor(t, c.target, c.src); !strings.Contains(build, "E1370") {
				t.Errorf("build does not report E1370 for the same module:\n%s", build)
			}
		})
	}
}

func TestCheckAcceptsWhatTheTargetProvides(t *testing.T) {
	t.Parallel()
	for name, target := range map[string]driver.Target{
		"hosted": driver.HostTarget(),
		"wasi":   mustTarget(t, "wasm32-wasi", ""),
	} {
		if out := checkFor(t, target, hostArgsSrc); out != "" {
			t.Errorf("%s: check rejected host.args:\n%s", name, out)
		}
	}
}

func TestCheckRejectsHostShellOnFreestanding(t *testing.T) {
	t.Parallel()
	out := checkFor(t, mustTarget(t, "x86_64-freestanding", ""), "let sh = host.shell()\nprint \"x\"\n")
	if !strings.Contains(out, "E100") || !strings.Contains(out, "host") {
		t.Fatalf("expected check to reject host on a freestanding target:\n%s", out)
	}
}
