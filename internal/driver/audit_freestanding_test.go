package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func writeAuditModule(t *testing.T, name, extraC string) string {
	t.Helper()
	root := t.TempDir()
	mod := "Module {\n    name: \"" + name + "\"\n    kigumi: \"0.1\"\n}\nSource { c: \"extra.c\" }\n"
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte(mod), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "extra.c"), []byte(extraC), 0o644)
	return root
}

// TestAuditRejectsDisallowedSymbol fails the build when it reaches an undeclared symbol like printf, instead of shipping the archive.
func TestAuditRejectsDisallowedSymbol(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := writeAuditModule(t, "leaky", "extern int printf(const char *fmt, ...);\nint leak(void) { return printf(\"hi\\n\"); }\n")
	target, err := driver.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "leaky.a")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, out, &buildErr, driver.BuildOptions{Target: target})
	if ok || err == nil {
		t.Fatalf("expected the build to fail on an undeclared printf, got ok=%v err=%v", ok, err)
	}
	if !strings.Contains(err.Error(), "printf") {
		t.Fatalf("expected the error to name printf, got: %v", err)
	}
	if !strings.Contains(err.Error(), out) {
		t.Fatalf("expected the error to quote the output path %s, got: %v", out, err)
	}
	if entries, _ := filepath.Glob(filepath.Join(root, "leaky.a*")); len(entries) != 0 {
		t.Fatalf("a failed audit left output behind: %v", entries)
	}
	if staged, _ := filepath.Glob(filepath.Join(root, ".leaky.a*")); len(staged) != 0 {
		t.Fatalf("a failed audit left a staged file behind: %v", staged)
	}
}

// TestAuditAllowsAllocatorNoneArchive: --allocator none (E959) is checker-only, since the runtime still needs malloc for strings and arenas.
func TestAuditAllowsAllocatorNoneArchive(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"allocnone\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	target, err := driver.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	target.Allocator = "none"
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "allocnone.a")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, out, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
}

// TestAuditRejectsTLS: a thread-local compiles fine but needs a TLS runtime freestanding does not have.
func TestAuditRejectsTLS(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := writeAuditModule(t, "tlsleak", "_Thread_local int counter;\nint bump(void) { return ++counter; }\n")
	target, err := driver.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "tlsleak.a")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, out, &buildErr, driver.BuildOptions{Target: target})
	if ok || err == nil {
		t.Fatalf("expected a thread-local symbol to fail the audit, got ok=%v err=%v", ok, err)
	}
	if !strings.Contains(err.Error(), "thread-local") {
		t.Fatalf("expected the error to call out TLS, got: %v", err)
	}
}

// TestBuildUEFITrivialProgram builds for x86_64-uefi without the embedder supplying COFF's compiler-inserted symbols (_fltused, __chkstk).
func TestBuildUEFITrivialProgram(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"uefitrivial\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("let x = 1\n"), 0o644)
	target, err := driver.ParseTarget(testCompiler(), "x86_64-uefi", "")
	if err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "uefitrivial.a")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, out, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
}

// TestAuditRejectsDisallowedSymbolUEFI is TestAuditRejectsDisallowedSymbol on the COFF/UEFI path, which carries extra compiler-inserted symbols.
func TestAuditRejectsDisallowedSymbolUEFI(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := writeAuditModule(t, "leakyuefi", "extern int printf(const char *fmt, ...);\nint leak(void) { return printf(\"hi\\n\"); }\n")
	target, err := driver.ParseTarget(testCompiler(), "x86_64-uefi", "")
	if err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "leakyuefi.a")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, out, &buildErr, driver.BuildOptions{Target: target})
	if ok || err == nil {
		t.Fatalf("expected the build to fail on an undeclared printf, got ok=%v err=%v", ok, err)
	}
	if !strings.Contains(err.Error(), "printf") {
		t.Fatalf("expected the error to name printf, got: %v", err)
	}
}

// TestAuditRejectsTLSUEFI is TestAuditRejectsTLS on COFF/UEFI, where TLS is read from the defining section's name, not a per-symbol type.
func TestAuditRejectsTLSUEFI(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := writeAuditModule(t, "tlsleakuefi", "_Thread_local int counter;\nint bump(void) { return ++counter; }\n")
	target, err := driver.ParseTarget(testCompiler(), "x86_64-uefi", "")
	if err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "tlsleakuefi.a")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, out, &buildErr, driver.BuildOptions{Target: target})
	if ok || err == nil {
		t.Fatalf("expected a thread-local symbol to fail the audit, got ok=%v err=%v", ok, err)
	}
	if !strings.Contains(err.Error(), "thread-local") {
		t.Fatalf("expected the error to call out TLS, got: %v", err)
	}
}

// TestParseTargetFreestandingSysContradiction: freestanding and UEFI targets have no OS, so --sys posix contradicts them.
func TestParseTargetFreestandingSysContradiction(t *testing.T) {
	t.Parallel()
	for _, triple := range []string{"x86_64-freestanding", "x86_64-uefi"} {
		if _, err := driver.ParseTarget(testCompiler(), triple, "posix"); err == nil {
			t.Errorf("%s: expected --sys posix to conflict with a freestanding target", triple)
		}
	}
	if _, err := driver.ParseTarget(testCompiler(), "x86_64-freestanding", "none"); err != nil {
		t.Fatalf("explicit --sys none should still work: %v", err)
	}
	if _, err := driver.ParseTarget(testCompiler(), "x86_64-linux", "posix"); err != nil {
		t.Fatalf("explicit --sys posix on a hosted target should still work: %v", err)
	}
}
