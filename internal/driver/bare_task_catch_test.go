package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"kigumi/internal/driver"
)

const bareHarness = `#include <stdlib.h>
#include <unistd.h>
extern int kigumi_main(int, char**);
void rt_sys_write(int fd, const char* p, unsigned long n) {
    while (n) {
        long w = write(fd, p, n);
        if (w <= 0) return;
        p += w;
        n -= (unsigned long)w;
    }
}
void rt_sys_exit(int code) { exit(code); }
int rt_sys_close(int fd) { return close(fd); }
int main(int argc, char** argv) { return kigumi_main(argc, argv); }
`

// A bare build jumps with the compiler's own setjmp, so a task panic still faults only the task.
func TestBareBuildContainsTaskPanic(t *testing.T) {
	cc := testCompiler().CCompiler()
	if cc == nil || runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("needs a C compiler on linux/amd64")
	}
	target, err := driver.ParseTarget(testCompiler(), "", "none")
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []string{
		"async_task_panic_second_fault",
		"async_task_panic_drop_panic",
		"async_task_panic_unwind",
		"async_task_panic_shared_with_mut",
		"async_task_cancel_drop_panic",
		"async_task_cancel_drop_panic_chain",
	} {
		t.Run(fixture, func(t *testing.T) {
			root, wantOut, wantErr, wantExit := loadRunFixture(t, "../../testdata/run/"+fixture+".txtar")
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../std", Entry: "main/main.kg", Target: target})
			if err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(root, "prog.a")
			var buildErr bytes.Buffer
			if ok, err := driver.BuildWith(m, archive, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
				t.Fatalf("build: %v\n%s", err, buildErr.String())
			}
			harness := filepath.Join(root, "harness.c")
			os.WriteFile(harness, []byte(bareHarness), 0o644)
			exe := filepath.Join(root, "prog")
			link := exec.Command(cc[0], append(cc[1:], harness, archive, "-lm", "-o", exe)...)
			if out, err := link.CombinedOutput(); err != nil {
				t.Fatalf("link: %v\n%s", err, out)
			}
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(exe)
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			exit := exitCodeOf(cmd.Run())
			if stdout.String() != wantOut || stderr.String() != wantErr || exit != wantExit {
				t.Fatalf("bare run differs from the corpus expectation\nstdout:\n%s\nwant:\n%s\nstderr:\n%s\nwant:\n%s\nexit %d, want %d",
					stdout.String(), wantOut, stderr.String(), wantErr, exit, wantExit)
			}
		})
	}
}
