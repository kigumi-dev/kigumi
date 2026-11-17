//go:build unix

package vm_test

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"kigumi/internal/backend/vm"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
)

func TestReadWriteDevNull(t *testing.T) {
	if os.Getenv("KIGUMI_TEST_DEVNULL") == "1" {
		res := checkSource(t, "let data = host.stdin().readAll()?\nprint \"${data.len()}\"\n")
		if res.HasErrors() {
			t.Fatal(res.Render())
		}
		prog := mirbuild.Program(res)
		if prog.HasErrors() {
			t.Fatal("ownership errors")
		}
		if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		code, err := vm.Run(prog, vm.Options{}, os.Stdout, &stderr)
		if err != nil || code != 0 || stderr.Len() != 0 {
			t.Fatalf("exit %d: %v\n%s", code, err, stderr.String())
		}
		return
	}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestReadWriteDevNull$")
	cmd.Env = append(os.Environ(), "KIGUMI_TEST_DEVNULL=1")
	cmd.Stdin, cmd.Stdout = null, null
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("child: %v\n%s", err, stderr.String())
	}
}
