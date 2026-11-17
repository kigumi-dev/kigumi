package driver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestUEFIDivisionHelperABI(t *testing.T) {
	requireZig(t)
	source, err := os.ReadFile("../../backend/llgen/runtimegen/runtime/rt_number.inc")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "#if UINTPTR_MAX < UINT64_MAX")
	end := strings.Index(text[start:], "static int64_t int_div") + start
	dir := t.TempDir()
	cpath, asmPath := filepath.Join(dir, "division.c"), filepath.Join(dir, "division.s")
	body := "#include <stdint.h>\n_Noreturn void rt_panic(const char *);\n" + text[start:end]
	if err := os.WriteFile(cpath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cc := testCompiler().CCompiler()
	args := append(append([]string{}, cc[1:]...), "-target", "x86-uefi", "-O1", "-S", cpath, "-o", asmPath)
	if out, err := exec.Command(cc[0], args...).CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	asm, err := os.ReadFile(asmPath)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"__alldiv", "__allrem", "__aulldiv", "__aullrem"}
	for _, name := range names {
		at := strings.Index(string(asm), "\n"+name+":")
		if at < 0 {
			t.Fatalf("missing %s in assembly", name)
		}
		body := string(asm)[at:]
		for _, next := range names {
			if end := strings.Index(body[1:], "\n"+next+":"); end >= 0 {
				body = body[:end+1]
			}
		}
		if !regexp.MustCompile(`(?m)^\s*retl\s+\$16\b`).MatchString(body) {
			t.Errorf("%s does not pop both 64-bit arguments:\n%s", name, body)
		}
	}
}
