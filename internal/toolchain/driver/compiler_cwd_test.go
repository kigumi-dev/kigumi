package driver_test

import (
	"bytes"
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func cwdModule(t *testing.T, o driver.CompilerOptions) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"cwdmod\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"cwd ok\"\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, Compiler: o})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBuildResolvesOutputAgainstCompilerCwd(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	cwd := t.TempDir()
	os.Mkdir(filepath.Join(cwd, "out"), 0o755)
	m := cwdModule(t, testCompilerAt(cwd))
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
	m := cwdModule(t, func() driver.CompilerOptions { o := testCompilerAt(t.TempDir()); o.PathDirs = nil; return o }())
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
	std, _ := filepath.Abs("../../../std")
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
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "ls.ld"), []byte("kigumi_script_marker = 0x1234;\n"), 0o644)
	m := cwdModule(t, testCompilerAt(cwd))
	var stderr bytes.Buffer
	ok, err := driver.BuildWith(m, "prog", &stderr, driver.BuildOptions{LinkerScript: "ls.ld"})
	if err != nil || !ok {
		t.Fatalf("link with the script: %v\n%s", err, stderr.String())
	}
	f, err := elf.Open(filepath.Join(cwd, "prog"))
	if err != nil {
		t.Skipf("not an ELF host: %v", err)
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range syms {
		if s.Name == "kigumi_script_marker" {
			return
		}
	}
	t.Fatal("the linker script's symbol is missing from the linked binary")
}

func TestBuildResolvesRelativeCFlagsAgainstCompilerCwd(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "extra.c"), []byte("__attribute__((used, retain)) int kigumi_extra_marker = 7;\n"), 0o644)
	m := cwdModule(t, testCompilerAt(cwd))
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	ok, err := driver.BuildWith(m, "prog", &stderr, driver.BuildOptions{Reproducible: true, CFlags: []string{"extra.c"}})
	if err != nil || !ok {
		t.Fatalf("build with a relative .c flag: %v\n%s", err, stderr.String())
	}
	f, err := elf.Open(filepath.Join(cwd, "prog"))
	if err != nil {
		t.Skipf("not an ELF host: %v", err)
	}
	defer f.Close()
	syms, _ := f.Symbols()
	for _, s := range syms {
		if s.Name == "kigumi_extra_marker" {
			return
		}
	}
	t.Fatal("the extra C file was not linked")
}

func TestLoadBuildProgramResolvesRelativeRootAgainstCompilerCwd(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	m := filepath.Join(b, "m")
	os.Mkdir(m, 0o755)
	os.WriteFile(filepath.Join(m, "mod.kg"), []byte("Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(m, "main.kg"), []byte("print \"x\"\n"), 0o644)
	os.WriteFile(filepath.Join(m, "build.kg"), []byte("fn configure(b: build.Builder) -> Void! {\n}\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	t.Chdir(a)
	cc := driver.CompilerOptions{Cwd: b, PathDirs: []string{}, Home: b}
	_, ok, err := driver.LoadBuildProgram("m", driver.LoadOptions{StdRoot: std, Compiler: cc})
	if !ok {
		t.Fatalf("build.kg under Cwd was not found: %v", err)
	}
}
