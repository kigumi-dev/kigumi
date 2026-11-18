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
    let which: i32 = match host.args().get(1) {
        Some("1") -> 1
        Some("2") -> 2
        Some("3") -> 3
        Some("4") -> 4
        Some("5") -> 5
        _ -> 0
    }
    unsafe { kg_fault(which) }
    print "survived"
}
`,
		"fault.c": `#define _GNU_SOURCE
#include <signal.h>
#include <string.h>
#include <sys/syscall.h>
#include <unistd.h>
void rt_sys_fault_cleanup(void) { (void)write(2, "cleanup\n", 8); }
static struct sigaction saved[2];
static void record(int sig, siginfo_t* si, void* ctx) {
    (void)write(2, si->si_code > 0 ? "hardware\n" : "sent\n", si->si_code > 0 ? 9 : 5);
    saved[sig == SIGBUS].sa_sigaction(sig, si, ctx);
    (void)write(2, "returned\n", 9);
}
static void record_all(void) {
    struct sigaction sa = {0};
    sa.sa_sigaction = record;
    sa.sa_flags = SA_SIGINFO | SA_ONSTACK | SA_NODEFER;
    sigaction(SIGSEGV, &sa, &saved[0]);
    sigaction(SIGBUS, &sa, &saved[1]);
}
void kg_fault(int which) {
    if (which < 2) raise(which ? SIGBUS : SIGSEGV);
    record_all();
    if (which == 3) kill(getpid(), SIGSEGV);
    if (which == 4) {
        siginfo_t si;
        memset(&si, 0, sizeof si);
        si.si_signo = SIGBUS;
        si.si_code = BUS_MCEERR_AO;
        syscall(SYS_rt_sigqueueinfo, getpid(), SIGBUS, &si);
    }
    if (which == 5) *(volatile long*)0x8000000000000000UL = 1;
    *(volatile int*)1 = 7;
}
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
	cases := []struct {
		name   string
		sig    syscall.Signal
		stderr string
	}{
		{"sent-SIGSEGV", syscall.SIGSEGV, "cleanup\n"},
		{"sent-SIGBUS", syscall.SIGBUS, "cleanup\n"},
		{"hardware-SIGSEGV", syscall.SIGSEGV, "hardware\ncleanup\nreturned\n"},
		{"kill-SIGSEGV", syscall.SIGSEGV, "sent\ncleanup\n"},
		{"queued-asynchronous-SIGBUS", syscall.SIGBUS, "hardware\ncleanup\n"},
		{"non-canonical-SIGSEGV", syscall.SIGSEGV, "hardware\ncleanup\nreturned\n"},
	}
	for i, tc := range cases {
		sig := tc.sig
		t.Run(tc.name, func(t *testing.T) {
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
			if stdout.Len() != 0 || stderr.String() != tc.stderr {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}
