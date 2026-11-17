package driver_test

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

func TestAotFatalSignalCleanup(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{
		"mod.kg": "Module { name: \"fault\", kigumi: \"0.1\" }\nSource { c: \"fault.c\" }\n",
		"main.kg": `import os from std/os
extern(C) { fn kg_fault(which: i32) }
fn main(host: os.Host) {
    let which: i32 = if host.args().get(1) == Some("1") { 1 } else { 0 }
    unsafe { kg_fault(which) }
    print "survived"
}
`,
		"fault.c": `#include <signal.h>
#include <unistd.h>
void rt_sys_fault_cleanup(void) { (void)write(2, "cleanup\n", 8); }
void kg_fault(int which) { raise(which ? SIGBUS : SIGSEGV); }
`,
	})
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "fault")
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	for i, sig := range []syscall.Signal{syscall.SIGSEGV, syscall.SIGBUS} {
		t.Run(sig.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, strconv.Itoa(i))
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if ctx.Err() != nil {
				t.Fatal("fatal signal hung")
			}
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ProcessState.Sys().(syscall.WaitStatus).Signal() != sig {
				t.Fatalf("exit=%v, want signal %v; stdout=%q stderr=%q", err, sig, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || stderr.String() != "cleanup\n" {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}
