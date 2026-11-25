//go:build unix

package vm_test

import (
	"bytes"
	"testing"

	"kigumi/internal/backend/vm"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
)

// TestSpawnMissingExecutableReports127 pins the fork+execvp emulation: a
// missing program is not a spawn error in a native build, where execvp fails
// in the child and the child exits 127, so the VM must return the same handle
// and exit status instead of surfacing Go's Start error.
func TestSpawnMissingExecutableReports127(t *testing.T) {
	src := "fn main() {\n" +
		"    let p = host.processes()\n" +
		"    match p.spawn(\"kigumi_no_such_command_aud16\", Array.empty[String]()) {\n" +
		"        Ok(child) -> {\n" +
		"            let mut c = child\n" +
		"            match c.wait() {\n" +
		"                Ok(code) -> print \"ok=${code}\"\n" +
		"                Err(e) -> print \"wait=${e.message()}\"\n" +
		"            }\n" +
		"        }\n" +
		"        Err(e) -> print \"spawn=${e.message()}\"\n" +
		"    }\n" +
		"}\n"
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
	code, err := vm.Run(prog, vm.Options{}, &out, &errOut)
	if err != nil {
		t.Fatalf("vm: %v\nstderr: %s", err, errOut.String())
	}
	if code != 0 {
		t.Fatalf("exit %d\nstderr: %s", code, errOut.String())
	}
	if want := "ok=127\n"; out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
}
