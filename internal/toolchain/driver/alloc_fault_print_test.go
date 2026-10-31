package driver_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

func buildFaultInjected(t *testing.T, m *driver.Module) string {
	t.Helper()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_FAULT_INJECT"}}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	return exe
}

func TestFaultInjectionPrintAllocatesNothing(t *testing.T) {
	t.Parallel()
	root, wantOut, wantErr, _ := loadRunFixture(t, "../../../testdata/run/print_write_path.txtar")
	src, err := os.ReadFile(filepath.Join(root, "main", "main.kg"))
	if err != nil {
		t.Fatal(err)
	}
	m := loadSweepModule(t, string(src), "recoverable-only")
	exe := buildFaultInjected(t, m)
	for n := 1; n <= 6; n++ {
		cmd := exec.Command(exe)
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if code := exitCodeOf(cmd.Run()); code != 0 || out.String() != wantOut || errOut.String() != wantErr {
			t.Errorf("native, refusing allocation %d: exit %d, %d of %d stdout bytes, stderr %q", n, code, out.Len(), len(wantOut), errOut.String())
		}
	}
	for i, code := range vmSweepExits(t, m) {
		if code != 0 {
			t.Errorf("vm, refusing allocation %d: exit %d", i+1, code)
		}
	}
}

// A signal handler and every panic reach the OS through these, so none of them may box.
func TestSysWritePathBoxesNothing(t *testing.T) {
	t.Parallel()
	m := loadSweepModule(t, "fn main() {\n    print \"x\"\n}\n", "recoverable-only")
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	for _, sym := range []string{"rt_sys_write", "rt_sys_exit", `"std/ffi.errno"`} {
		body := defineBlock(ir, sym)
		if body == "" {
			t.Fatalf("no define for %s", sym)
		}
		for _, alloc := range []string{"@rt_int(", "@rt_ptr(", "@rt_float(", "@rt_str", "@rt_record(", "@malloc(", "@calloc("} {
			if strings.Contains(body, alloc) {
				t.Errorf("%s allocates through %s:\n%s", sym, alloc, body)
			}
		}
	}
}

const stackOverflowArmedSrc = `fn depth(n: Int) -> Int {
    1 + depth(n + 1)
}

fn main() {
    eprint "start"
    print "${depth(0)}"
}
`

const indexPanicArmedSrc = `fn at(xs: &Array[Int], i: usize) -> Int {
    xs[i]
}

fn main() {
    let xs = Array.of(1, 2, 3)
    eprint "start"
    let v = at(&xs, 7)
    print "${v}"
}
`

// Refusing the nth allocation after the arming eprint (n = 1..3) still leaves the stack guard's panic and rt_fatal their own message.
func TestFaultInjectionPanicMessageAllocatesNothing(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, src, want string }{
		{"stack overflow", stackOverflowArmedSrc, "panic: stack overflow\n"},
		{"index out of range", indexPanicArmedSrc, "panic: index out of range: index 7, length 3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			exe := buildFaultInjected(t, loadSweepModule(t, c.src, ""))
			for n := 1; n <= 3; n++ {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				cmd := exec.CommandContext(ctx, exe)
				cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_ARM=1", "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
				var errOut bytes.Buffer
				cmd.Stderr = &errOut
				code := exitCodeOf(cmd.Run())
				hung := ctx.Err() != nil
				cancel()
				if hung {
					t.Fatalf("refusing allocation %d: the program hung", n)
				}
				if got := errOut.String(); code != 2 || !strings.Contains(got, c.want) || strings.Contains(got, "out of memory") {
					t.Errorf("refusing allocation %d: exit %d, stderr %q, want exit 2 and %q", n, code, got, c.want)
				}
			}
		})
	}
}
