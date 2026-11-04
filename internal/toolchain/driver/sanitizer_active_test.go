package driver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestSanitizerRuntimeIsActive proves the C compiler the other tests use really links a sanitizer
// runtime when KIGUMI_CFLAGS asks for one. A compiler that drops the flag would let the corpus pass
// with no instrumentation, which is worse than not running the sanitizer step at all.
func TestSanitizerRuntimeIsActive(t *testing.T) {
	want := slices.ContainsFunc(strings.Fields(os.Getenv("KIGUMI_CFLAGS")), func(f string) bool {
		return strings.HasPrefix(f, "-fsanitize=") && strings.Contains(f, "address")
	})
	if !want {
		t.Skip("set KIGUMI_CFLAGS=-fsanitize=address to prove the sanitizer runtime")
	}
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("no C compiler")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "overflow.c")
	if err := os.WriteFile(src, []byte("#include <stdlib.h>\nint main(void) {\n    volatile char *p = malloc(4);\n    p[4] = 1;\n    free((void *)p);\n    return 0;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "overflow")
	args := append(append([]string{}, cc[1:]...), "-g", "-fsanitize=address", src, "-o", exe)
	if out, err := exec.Command(cc[0], args...).CombinedOutput(); err != nil {
		t.Fatalf("%s cannot build with -fsanitize=address: %v\n%s", strings.Join(cc, " "), err, out)
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "AddressSanitizer") {
		t.Fatalf("the overflow was not reported (err=%v):\n%s", err, out)
	}
}
