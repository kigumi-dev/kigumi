package driver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// Compiling the shipped text, not a copy, keeps the probe from drifting from the runtime.
func extractCFunc(t *testing.T, src, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)\n[\w* ]*\b` + regexp.QuoteMeta(name) + `\([^;{]*\)\s*\{.*?\n\}\n`)
	m := re.FindString(src)
	if m == "" {
		t.Fatalf("could not find %s in the runtime source (has its text changed shape?)", name)
	}
	return m
}

func compileToAsm(t *testing.T, dir, name, src string) string {
	t.Helper()
	cc := testCompiler().CCompiler()
	cPath := filepath.Join(dir, name+".c")
	sPath := filepath.Join(dir, name+".s")
	if err := os.WriteFile(cPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	args := append(cc[1:], "-O2", "-S", "-Wno-override-module", cPath, "-o", sPath)
	out, err := exec.Command(cc[0], args...).CombinedOutput()
	if err != nil {
		t.Fatalf("compile %s: %v\n%s", name, err, out)
	}
	asm, err := os.ReadFile(sPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(asm)
}

func asmFunc(asm, label string) string {
	start := strings.Index(asm, "\n"+label+":")
	if start < 0 {
		return ""
	}
	rest := asm[start:]
	end := strings.Index(rest, ".cfi_endproc")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// Compiles rt_secure_zero into a dead-store-shaped probe and checks -O2
// keeps its stores; a non-volatile control confirms -O2 would elide an unprotected one.
func TestSecureEraseSurvivesO2DeadStoreElimination(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	if runtime.GOARCH != "amd64" || runtime.GOOS != "linux" {
		t.Skip("x86-64 Linux only: the test reads AT&T assembly mnemonics")
	}
	rtCore, err := os.ReadFile("../../backend/llgen/runtime/rt_cap_mem_secure_erase.inc")
	if err != nil {
		t.Fatal(err)
	}
	secureZero := extractCFunc(t, string(rtCore), "rt_secure_zero")

	src := "#include <stdint.h>\n" +
		"void opaque_use(unsigned char* p) { __asm__ volatile(\"\" : : \"r\"(p) : \"memory\"); }\n" +
		"static " + secureZero +
		"void probe_protected(void) {\n" +
		"    unsigned char buf[64];\n" +
		"    opaque_use(buf);\n" +
		"    rt_secure_zero((volatile unsigned char*)buf, 64);\n" +
		"}\n" +
		"void probe_unprotected(void) {\n" +
		"    unsigned char buf[64];\n" +
		"    opaque_use(buf);\n" +
		"    unsigned char* p = buf;\n" +
		"    for (long long i = 0; i < 64; i++) p[i] = 0;\n" +
		"}\n"
	asm := compileToAsm(t, t.TempDir(), "secure_erase", src)

	protected := asmFunc(asm, "probe_protected")
	if protected == "" {
		t.Fatal("probe_protected not found in the compiled assembly")
	}
	if !strings.Contains(protected, "movb\t$0,") && !strings.Contains(protected, "movb $0,") {
		t.Errorf("rt_secure_zero's stores were optimized away at -O2:\n%s", protected)
	}

	unprotected := asmFunc(asm, "probe_unprotected")
	if unprotected == "" {
		t.Fatal("probe_unprotected not found in the compiled assembly")
	}
	if strings.Contains(unprotected, "movb\t$0,") || strings.Contains(unprotected, "movb $0,") {
		t.Skip("this toolchain did not eliminate the unprotected control loop at -O2; the DSE scenario this test checks did not occur, so it proves nothing either way")
	}
}

// branchAfterSecretLoad reports whether asm compares a byte loaded through
// rdi/rdx (the two secret-buffer args) and then branches on it.
func branchAfterSecretLoad(asm string) bool {
	cmpRe := regexp.MustCompile(`^\s*(cmp|test)\w*\s.*\((%rdi|%rdx)[,)]`)
	jRe := regexp.MustCompile(`^\s*(j\w+)\s`)
	skipRe := regexp.MustCompile(`^\s*(\.|#|$|\S+:\s*$)`)
	lines := strings.Split(asm, "\n")
	for i, l := range lines {
		if !cmpRe.MatchString(l) {
			continue
		}
		for j := i + 1; j < len(lines) && j < i+6; j++ {
			if skipRe.MatchString(lines[j]) {
				continue
			}
			if m := jRe.FindStringSubmatch(lines[j]); m != nil && m[1] != "jmp" {
				return true
			}
			break
		}
	}
	return false
}

// Compiles the real subtle_constant_time_eq and checks -O2 kept it branch-free
// on a loaded secret byte; a naive early-exit comparison is the control.
func TestSubtleConstantTimeEqHasNoSecretDependentBranch(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	if runtime.GOARCH != "amd64" || runtime.GOOS != "linux" {
		t.Skip("x86-64 Linux only: the test reads AT&T assembly mnemonics")
	}
	rtHeader, err := os.ReadFile("../../backend/llgen/runtime/rt.h")
	if err != nil {
		t.Fatal(err)
	}
	rtCore, err := os.ReadFile("../../backend/llgen/runtime/rt_utf8.inc")
	if err != nil {
		t.Fatal(err)
	}
	ctEq := extractCFunc(t, string(rtCore), "subtle_constant_time_eq")

	ctSrc := string(rtHeader) + "\n" + ctEq +
		"int ct_wrapper(const unsigned char* a, long long n, const unsigned char* b, long long blen) {\n" +
		"    struct obj oa = {0}, ob = {0};\n" +
		"    oa.kind = K_BYTES; oa.u.s.data = (const char*)a; oa.u.s.len = n;\n" +
		"    ob.kind = K_BYTES; ob.u.s.data = (const char*)b; ob.u.s.len = blen;\n" +
		"    return subtle_constant_time_eq(&oa, &ob);\n" +
		"}\n"
	ctAsm := compileToAsm(t, t.TempDir(), "constant_time_real", ctSrc)
	if branchAfterSecretLoad(ctAsm) {
		t.Errorf("subtle_constant_time_eq's loop branches on a loaded secret byte at -O2:\n%s", ctAsm)
	}

	naiveSrc := "int naive_eq(const unsigned char* a, long long n, const unsigned char* b, long long blen) {\n" +
		"    if (n != blen) return 0;\n" +
		"    for (long long i = 0; i < n; i++) {\n" +
		"        if (a[i] != b[i]) return 0;\n" +
		"    }\n" +
		"    return 1;\n" +
		"}\n"
	naiveAsm := compileToAsm(t, t.TempDir(), "constant_time_naive", naiveSrc)
	if !branchAfterSecretLoad(naiveAsm) {
		t.Skip("this toolchain did not compile the naive control to a secret-dependent branch either; the check below is not discriminating on this toolchain, so it proves nothing either way")
	}
}
