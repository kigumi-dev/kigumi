package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// embedderGCStub is a minimal libc a freestanding embedder must supply
// (linking is the embedder's step), plus a
// _start that calls kigumi_main and exits. It exists only to let this
// test produce a real ELF; the bodies need not be correct.
const embedderGCStub = `#include <stddef.h>
extern int kigumi_main(int argc, char **argv);
int strncmp(const char *a, const char *b, size_t n) { (void)a; (void)b; (void)n; return 0; }
int strcmp(const char *a, const char *b) { (void)a; (void)b; return 0; }
char *strstr(const char *h, const char *n) { (void)h; (void)n; return (void*)0; }
char *strchr(const char *s, int c) { (void)s; (void)c; return (void*)0; }
char *strrchr(const char *s, int c) { (void)s; (void)c; return (void*)0; }
size_t strlen(const char *s) { size_t n = 0; while (s[n]) n++; return n; }
int atoi(const char *s) { (void)s; return 0; }
double pow(double a, double b) { (void)a; (void)b; return 0; }
double ceil(double a) { return a; }
double floor(double a) { return a; }
double round(double a) { return a; }
double sqrt(double a) { return a; }
double trunc(double a) { return a; }
double strtod(const char *s, char **end) { (void)s; if (end) *end = (char*)s; return 0; }
float strtof(const char *s, char **end) { (void)s; if (end) *end = (char*)s; return 0; }
int snprintf(char *buf, size_t n, const char *fmt, ...) { (void)buf; (void)n; (void)fmt; return 0; }
void *memcpy(void *d, const void *s, size_t n) { unsigned char *dd = d; const unsigned char *ss = s; for (size_t i = 0; i < n; i++) dd[i] = ss[i]; return d; }
void *memset(void *d, int c, size_t n) { unsigned char *dd = d; for (size_t i = 0; i < n; i++) dd[i] = (unsigned char)c; return d; }
int memcmp(const void *a, const void *b, size_t n) { (void)a; (void)b; (void)n; return 0; }
static unsigned char gc_stub_heap[1 << 20];
static size_t gc_stub_heap_off;
void *malloc(size_t n) {
    n = (n + 15) & ~(size_t)15;
    if (gc_stub_heap_off + n > sizeof(gc_stub_heap)) return (void*)0;
    void *p = &gc_stub_heap[gc_stub_heap_off];
    gc_stub_heap_off += n;
    return p;
}
void *calloc(size_t n, size_t sz) { void *p = malloc(n * sz); if (p) memset(p, 0, n * sz); return p; }
void *realloc(void *p, size_t n) { (void)p; return malloc(n); }
void free(void *p) { (void)p; }
static int gc_stub_errno;
int *__errno_location(void) { return &gc_stub_errno; }
void _start(void) {
    kigumi_main(0, (void*)0);
    __asm__ volatile("mov $60, %%rax\n\txor %%rdi, %%rdi\n\tsyscall" ::: "rax", "rdi");
    __builtin_unreachable();
}
`

// -ffunction-sections/-fdata-sections still matters for symbols reachability itself cannot prune.
func TestBuildArchiveFunctionSections(t *testing.T) {
	t.Parallel()
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("no C compiler")
	}
	for _, tool := range []string{"ar", "readelf"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"fsects\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hello\"\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "fsects.a")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, archive, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}

	extractDir := t.TempDir()
	cmd := exec.Command("ar", "x", archive)
	cmd.Dir = extractDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ar x: %v\n%s", err, out)
	}
	obj := filepath.Join(extractDir, "obj0.o")
	sections, err := exec.Command("readelf", "-W", "-S", obj).CombinedOutput()
	if err != nil {
		t.Fatalf("readelf -S: %v\n%s", err, sections)
	}
	if !strings.Contains(string(sections), ".text.") {
		t.Fatalf("expected per-function .text.* sections in the module object, got:\n%s", sections)
	}
}

// An unused import contributes no symbols at all, with no embedder --gc-sections needed.
func TestBuildArchiveReachabilityPrunesUnusedImport(t *testing.T) {
	t.Parallel()
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("no C compiler")
	}
	if _, err := exec.LookPath("nm"); err != nil {
		t.Skip("no nm")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"unused\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("import json from std/json\nprint \"hello\"\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "unused.a")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, archive, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}

	stub := filepath.Join(root, "embedder_stub.c")
	os.WriteFile(stub, []byte(embedderGCStub), 0o644)
	bin := filepath.Join(root, "linked")
	// zig cc instruments C with UBSan by default; without a libc there is no handler runtime to link.
	args := append(append([]string{}, cc[1:]...), "-target", "x86_64-freestanding", "-nostdlib", "-static", "-fno-sanitize=undefined", "-o", bin, stub, archive)
	linkCmd := exec.Command(cc[0], args...)
	var linkErr bytes.Buffer
	linkCmd.Stderr = &linkErr
	if err := linkCmd.Run(); err != nil {
		t.Fatalf("link: %v\n%s", err, linkErr.String())
	}
	out, err := exec.Command("nm", "--defined-only", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("nm: %v\n%s", err, out)
	}
	if n := codeSymbolsOf(string(out), "std/json."); n != 0 {
		t.Fatalf("expected an unused std/json import to contribute no function symbols without --gc-sections, got %d:\n%s", n, out)
	}
}

// T/t are code symbols; type descriptors are data symbols and always linked, so excluded here.
func codeSymbolsOf(nmOut, substr string) int {
	n := 0
	for _, line := range strings.Split(nmOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if (fields[1] == "T" || fields[1] == "t") && strings.Contains(fields[2], substr) {
			n++
		}
	}
	return n
}

// TestBuildArchiveCFlagsLinkerFlagsIgnored:
// a linker flag in KIGUMI_CFLAGS must not reach buildArchive's -c-only
// compile step, since zig cc silently corrupts the object otherwise.
func TestBuildArchiveCFlagsLinkerFlagsIgnored(t *testing.T) {
	t.Parallel()
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("no C compiler")
	}
	for _, tool := range []string{"ar", "nm"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"cflags\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hello\"\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "cflags.a")
	var buildErr bytes.Buffer
	opts := driver.BuildOptions{Target: target, CFlags: []string{"-Wl,--gc-sections"}}
	if ok, err := driver.BuildWith(m, archive, &buildErr, opts); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}

	extractDir := t.TempDir()
	cmd := exec.Command("ar", "x", archive)
	cmd.Dir = extractDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ar x: %v\n%s", err, out)
	}
	obj := filepath.Join(extractDir, "obj0.o")
	out, err := exec.Command("nm", "--defined-only", obj).CombinedOutput()
	if err != nil {
		t.Fatalf("nm: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatalf("obj0.o has no defined symbols: -Wl,--gc-sections from CFlags reached the compile-only step and corrupted it")
	}
}
