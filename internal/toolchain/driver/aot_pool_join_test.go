package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A worker left running at exit is a thread leak under ThreadSanitizer.
func TestAotAbortedRootJoinsOrphanedPool(t *testing.T) {
	if testCompiler().CCompiler() == nil || runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("needs a C compiler and ThreadSanitizer on linux/amd64")
	}
	for fixture, panicText := range map[string]string{
		"async_thread_cancel_with_orphan":     "panic: the root task was cancelled",
		"async_thread_fault_with_orphan":      "panic: boom",
		"async_thread_fault_cancels_siblings": "panic: boom",
		"async_thread_cancel_live_pool":       "panic: the root task was cancelled",
	} {
		t.Run(fixture, func(t *testing.T) { runJoinCheck(t, fixture, panicText) })
	}
}

func runJoinCheck(t *testing.T, fixture, panicText string) {
	root, _, _, _ := loadRunFixture(t, "../../../testdata/run/"+fixture+".txtar")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-fsanitize=thread", "-g"}})
	if err != nil || !ok {
		t.Skipf("no ThreadSanitizer runtime: %v\n%s", err, buildErr.String())
	}
	for i := range 3 {
		var errOut bytes.Buffer
		cmd := exec.Command(exe)
		cmd.Stderr = &errOut
		_ = cmd.Run()
		if !strings.Contains(errOut.String(), panicText) {
			t.Fatalf("run %d: stderr lacks %q:\n%s", i, panicText, errOut.String())
		}
		if strings.Contains(errOut.String(), "thread leak") {
			t.Fatalf("run %d: a pool worker outlived the run:\n%s", i, errOut.String())
		}
	}
}

const joinCountHook = `#include <pthread.h>
#include <unistd.h>
static inline int kg_counted_join(pthread_t t, void** r) {
    (void)!write(2, "joined\n", 7);
    return (pthread_join)(t, r);
}
#define pthread_join(t, r) kg_counted_join(t, r)
`

func TestAotAbortedRootJoinsEveryWorker(t *testing.T) {
	if testCompiler().CCompiler() == nil || runtime.GOOS != "linux" {
		t.Skip("needs a C compiler and pthreads")
	}
	for _, c := range []struct {
		fixture, panicText string
		workers            int
	}{
		{"async_thread_cancel_with_orphan", "panic: the root task was cancelled", 4},
		{"async_thread_fault_with_orphan", "panic: boom", 4},
		{"async_thread_fault_cancels_siblings", "panic: boom", 3},
		{"async_thread_cancel_live_pool", "panic: the root task was cancelled", 3},
	} {
		t.Run(c.fixture, func(t *testing.T) {
			root, _, _, _ := loadRunFixture(t, "../../../testdata/run/"+c.fixture+".txtar")
			hook := filepath.Join(root, "join_hook.h")
			if err := os.WriteFile(hook, []byte(joinCountHook), 0o644); err != nil {
				t.Fatal(err)
			}
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
			if err != nil {
				t.Fatal(err)
			}
			exe := filepath.Join(root, "prog")
			var buildErr bytes.Buffer
			if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-include", hook}}); err != nil || !ok {
				t.Fatalf("build: %v\n%s", err, buildErr.String())
			}
			var errOut bytes.Buffer
			cmd := exec.Command(exe)
			cmd.Stderr = &errOut
			_ = cmd.Run()
			got := errOut.String()
			if n := strings.Count(got, "joined\n"); n != c.workers {
				t.Fatalf("%d workers joined, want %d:\n%s", n, c.workers, got)
			}
			if !strings.HasSuffix(got, c.panicText+"\n") {
				t.Fatalf("stderr does not end with %q:\n%s", c.panicText, got)
			}
		})
	}
}
