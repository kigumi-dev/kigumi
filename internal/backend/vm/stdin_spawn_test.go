package vm_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
)

// runVMProgram compiles src and runs it with opts, returning the program output
// and exit code.
func runVMProgram(t *testing.T, src string, opts vm.Options) (string, string, int) {
	t.Helper()
	res := checkSource(t, src)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	if prog.HasErrors() {
		t.Fatal("ownership errors")
	}
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
		t.Fatalf("plan-rc: %v", err)
	}
	var out, errOut bytes.Buffer
	code, err := vm.Run(prog, opts, &out, &errOut)
	if err != nil {
		t.Fatalf("vm: %v", err)
	}
	return out.String(), errOut.String(), code
}

// TestStdinReadThenSpawnSeesRemainingInput pins that a readLine consumes only
// its own line: a child spawned afterwards inherits the same stdin and reads
// the rest, as native's one-byte line reader leaves the descriptor positioned.
func TestStdinReadThenSpawnSeesRemainingInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX sh")
	}
	outPath := filepath.Join(t.TempDir(), "child.out")
	src := fmt.Sprintf("import proc from std/proc\n"+
		"fn main() -> Void! {\n"+
		"    match host.stdin().readLine()? {\n"+
		"        Some(s) -> print \"first=[${s}]\"\n"+
		"        None -> print \"first=<none>\"\n"+
		"    }\n"+
		"    let p = host.processes()\n"+
		"    let code = p.run(\"sh\", Array.of(\"-c\", \"cat > %s\"))?\n"+
		"    print \"child=${code}\"\n"+
		"}\n", outPath)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := w.WriteString("one\ntwo\nthree\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()

	out, errOut, code := runVMProgram(t, src, vm.Options{Stdin: r})
	if code != 0 {
		t.Fatalf("exit %d\nstderr: %s", code, errOut)
	}
	if want := "first=[one]\nchild=0\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("child output: %v", err)
	}
	if want := "two\nthree\n"; string(data) != want {
		t.Errorf("child read %q, want %q", data, want)
	}
}

// TestStdinReadHugeSizeDoesNotAbort checks a giant Stdin.read size is refused
// or served from a bounded buffer instead of make() aborting the VM, matching
// the hosted runtime's malloc refusal/read semantics.
func TestStdinReadHugeSizeDoesNotAbort(t *testing.T) {
	src := "fn main() -> Void! {\n" +
		"    let mut arr = Array.empty[u8]()\n" +
		"    let mut si = host.stdin()\n" +
		"    match si.read(&mut arr, 3221225472) {\n" +
		"        Ok(n) -> print \"read=ok ${n}\"\n" +
		"        Err(e) -> print \"read=err [${e.message()}]\"\n" +
		"    }\n" +
		"    match si.read(&mut arr, 34359738368) {\n" +
		"        Ok(n) -> print \"toobig=ok ${n}\"\n" +
		"        Err(e) -> print \"toobig=err [${e.message()}]\"\n" +
		"    }\n" +
		"}\n"

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	w.Close()

	out, errOut, code := runVMProgram(t, src, vm.Options{Stdin: r})
	if code != 0 {
		t.Fatalf("exit %d\nstderr: %s", code, errOut)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 || lines[0] != "read=ok 0" {
		t.Fatalf("stdout = %q, want an empty first read on closed stdin", out)
	}
	if !strings.HasPrefix(lines[1], "toobig=err") {
		t.Errorf("second read = %q, want a recoverable out-of-memory error", lines[1])
	}
}
