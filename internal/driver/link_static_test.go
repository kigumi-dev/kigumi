package driver_test

import (
	"bytes"
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func buildSrcWith(t *testing.T, src string, opts driver.BuildOptions) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"linkform\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	return buildRootWith(t, root, opts)
}

func buildRootWith(t *testing.T, root string, opts driver.BuildOptions) string {
	t.Helper()
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, opts); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	return exe
}

type elfFacts struct {
	interp, symtab bool
	libs           []string
}

func readELF(t *testing.T, exe string) elfFacts {
	t.Helper()
	f, err := elf.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var facts elfFacts
	for _, p := range f.Progs {
		facts.interp = facts.interp || p.Type == elf.PT_INTERP
	}
	facts.symtab = f.Section(".symtab") != nil
	facts.libs, _ = f.ImportedLibraries()
	return facts
}

func runExe(t *testing.T, exe string) string {
	t.Helper()
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	return string(out)
}

// The host default is one static binary; --link dynamic keeps glibc and links libm only when the program reaches it.
func TestHostLinkForms(t *testing.T) {
	if runtime.GOOS != "linux" || testCompiler().CCompiler() == nil {
		t.Skip("needs a Linux host and a C compiler")
	}
	const plain = "print \"hello\"\n"
	const pow = "import math from std/math\nlet x = 2.0\nprint \"${math.pow(x, 10.0)}\"\n"
	t.Run("default", func(t *testing.T) {
		var r driver.BuildReport
		exe := buildSrcWith(t, plain, driver.BuildOptions{Report: &r})
		if facts := readELF(t, exe); facts.interp || len(facts.libs) != 0 {
			t.Errorf("default build is not static: %+v", facts)
		}
		if !r.Linkage.Static || r.Linkage.Reason == "" {
			t.Errorf("report = %+v, want a static linkage with its reason", r)
		}
		if got := runExe(t, exe); got != "hello\n" {
			t.Errorf("output %q", got)
		}
	})
	t.Run("dynamic without libm", func(t *testing.T) {
		exe := buildSrcWith(t, plain, driver.BuildOptions{Link: driver.LinkDynamic})
		facts := readELF(t, exe)
		if !facts.interp || slices.Contains(facts.libs, "libm.so.6") {
			t.Errorf("--link dynamic hello: %+v, want an interpreter and no libm", facts)
		}
	})
	t.Run("dynamic with libm", func(t *testing.T) {
		exe := buildSrcWith(t, pow, driver.BuildOptions{Link: driver.LinkDynamic})
		if facts := readELF(t, exe); !slices.Contains(facts.libs, "libm.so.6") {
			t.Errorf("math.pow built --link dynamic needs libm: %+v", facts)
		}
		if got := runExe(t, exe); got != "1024\n" {
			t.Errorf("output %q", got)
		}
	})
	t.Run("size-min", func(t *testing.T) {
		exe := buildSrcWith(t, plain, driver.BuildOptions{SizeMin: true})
		if facts := readELF(t, exe); facts.interp || facts.symtab {
			t.Errorf("--size-min hello: %+v, want static and stripped", facts)
		}
		if got := runExe(t, exe); got != "hello\n" {
			t.Errorf("output %q", got)
		}
	})
	t.Run("extern dlopen stays dynamic", func(t *testing.T) {
		const src = "import ffi from std/ffi\n\nextern(C) {\n    fn dlopen(name: *const u8, flags: i32) -> *mut u8\n}\n\n" +
			"let lib = \"libm.so.6\"\nlet name = ffi.CString.new(&lib)?\nlet h = unsafe {\n    dlopen(name.ptr(), 2)\n}\nprint \"null=${ffi.isNull(ffi.toConst(h))}\"\n"
		var r driver.BuildReport
		exe := buildSrcWith(t, src, driver.BuildOptions{Report: &r})
		if r.Linkage.Static || !strings.Contains(r.Linkage.Reason, "extern(C) dlopen") {
			t.Errorf("report = %+v, want the host C library for extern(C) dlopen", r)
		}
		if got := runExe(t, exe); got != "null=false\n" {
			t.Errorf("output %q", got)
		}
	})
	t.Run("glibc-only symbol names --link dynamic", func(t *testing.T) {
		root := t.TempDir()
		os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"linkform\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
		os.WriteFile(filepath.Join(root, "main.kg"), []byte("extern(C) {\n    fn gnu_get_libc_version() -> *const u8\n}\n\nlet v = unsafe {\n    gnu_get_libc_version()\n}\n"), 0o644)
		std, _ := filepath.Abs("../../std")
		m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
		if err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		_, err = driver.BuildWith(m, filepath.Join(t.TempDir(), "prog"), &stderr, driver.BuildOptions{})
		if err == nil || !strings.Contains(err.Error(), "--link dynamic") {
			t.Errorf("err = %v, want a link failure that names --link dynamic\n%s", err, stderr.String())
		}
	})
}
