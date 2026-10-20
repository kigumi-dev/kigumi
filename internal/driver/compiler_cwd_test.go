package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func cwdModule(t *testing.T, o driver.CompilerOptions) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"cwdmod\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"cwd ok\"\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, Compiler: o})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBuildResolvesOutputAgainstCompilerCwd(t *testing.T) {
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	cwd := t.TempDir()
	os.Mkdir(filepath.Join(cwd, "out"), 0o755)
	m := cwdModule(t, driver.CompilerOptions{Cwd: cwd})
	var stderr bytes.Buffer
	ok, err := driver.BuildWith(m, filepath.Join("out", "prog"), &stderr, driver.BuildOptions{})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, stderr.String())
	}
	exe := filepath.Join(cwd, "out", "prog")
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil || string(out) != "cwd ok\n" {
		t.Fatalf("run %s: %q %v", exe, out, err)
	}
	if _, err := os.Stat(filepath.Join("out", "prog")); err == nil {
		t.Fatal("the output also landed under the process cwd")
	}
}

func TestBuildFindsToolsOnlyInCompilerPathDirs(t *testing.T) {
	m := cwdModule(t, driver.CompilerOptions{Cwd: t.TempDir(), PathDirs: []string{}})
	ok, err := driver.BuildWith(m, "prog", &bytes.Buffer{}, driver.BuildOptions{})
	if ok || err == nil || !strings.Contains(err.Error(), "no C compiler found") {
		t.Fatalf("an empty PathDirs must find no compiler: %v %v", ok, err)
	}
}

func TestLoadModuleResolvesRelativeRootAgainstCompilerCwd(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	os.Mkdir(filepath.Join(a, "m"), 0o755)
	os.WriteFile(filepath.Join(a, "m", "mod.kg"), []byte("Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(a, "m", "main.kg"), []byte("print \"x\"\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	t.Chdir(a)
	if _, err := driver.LoadModule("m", driver.LoadOptions{StdRoot: std, Compiler: driver.CompilerOptions{Cwd: b}}); err == nil {
		t.Fatal("a root that is absent under Cwd must not be read from the process cwd")
	}
	m, err := driver.LoadModule("m", driver.LoadOptions{StdRoot: std, Compiler: driver.CompilerOptions{Cwd: a}})
	if err != nil || m.Root != "m" {
		t.Fatalf("same directory keeps the root as given: %v %v", m, err)
	}
	os.Rename(filepath.Join(a, "m"), filepath.Join(b, "m"))
	m, err = driver.LoadModule("m", driver.LoadOptions{StdRoot: std, Compiler: driver.CompilerOptions{Cwd: b}})
	if err != nil || m.Root != filepath.Join(b, "m") {
		t.Fatalf("root under Cwd: %v %v", m, err)
	}
}

func TestBuildResolvesLinkerScriptAgainstCompilerCwd(t *testing.T) {
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "ls.ld"), []byte("SECTIONS { }\n"), 0o644)
	m := cwdModule(t, driver.CompilerOptions{Cwd: cwd})
	var stderr bytes.Buffer
	_, _ = driver.BuildWith(m, "prog", &stderr, driver.BuildOptions{LinkerScript: "ls.ld"})
	if strings.Contains(stderr.String(), "cannot find linker script") {
		t.Fatalf("script not resolved against Cwd:\n%s", stderr.String())
	}
}

func TestLoadBuildProgramResolvesRelativeRootAgainstCompilerCwd(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	m := filepath.Join(b, "m")
	os.Mkdir(m, 0o755)
	os.WriteFile(filepath.Join(m, "mod.kg"), []byte("Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(m, "main.kg"), []byte("print \"x\"\n"), 0o644)
	os.WriteFile(filepath.Join(m, "build.kg"), []byte("fn configure(b: build.Builder) -> Unit! {\n}\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	t.Chdir(a)
	cc := driver.CompilerOptions{Cwd: b, PathDirs: []string{}, Home: b}
	_, ok, err := driver.LoadBuildProgram("m", driver.LoadOptions{StdRoot: std, Compiler: cc})
	if !ok {
		t.Fatalf("build.kg under Cwd was not found: %v", err)
	}
}
