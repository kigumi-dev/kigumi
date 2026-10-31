package driver_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

// awaitLoopModule is async_await_release_loop with n iterations; a parent that kept every awaited child would grow with n.
func awaitLoopModule(t *testing.T, n string) *driver.Module {
	root, _, _, _ := loadRunFixture(t, "../../../testdata/run/async_await_release_loop.txtar")
	main := filepath.Join(root, "main", "main.kg")
	src, err := os.ReadFile(main)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(main, []byte(strings.Replace(string(src), "20000", n, 1)), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestAotAwaitedChildrenStayBounded runs the program from a fresh copy of the test binary: a child's ru_maxrss also counts the peak of the process that forked it.
func TestAotAwaitedChildrenStayBounded(t *testing.T) {
	if exe := os.Getenv("KIGUMI_RSS_PROBE"); exe != "" {
		cmd := exec.Command(exe)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		fmt.Printf("rss-probe %d %q\n", cmd.ProcessState.SysUsage().(*syscall.Rusage).Maxrss, out)
		return
	}
	if testCompiler().CCompiler() == nil || runtime.GOOS != "linux" {
		t.Skip("needs a C compiler and getrusage on linux")
	}
	m := awaitLoopModule(t, "3000000")
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	probe := exec.Command(os.Args[0], "-test.run=^TestAotAwaitedChildrenStayBounded$", "-test.count=1")
	probe.Env = append(os.Environ(), "KIGUMI_RSS_PROBE="+exe)
	report, err := probe.CombinedOutput()
	var rss int64
	var out string
	if i := bytes.Index(report, []byte("rss-probe ")); err != nil || i < 0 {
		t.Fatalf("probe: %v\n%s", err, report)
	} else if _, err := fmt.Sscanf(string(report[i:]), "rss-probe %d %q", &rss, &out); err != nil {
		t.Fatalf("probe output: %v\n%s", err, report)
	}
	if out != "total 3000000\n" {
		t.Fatalf("stdout %q", out)
	}
	t.Logf("max RSS %d KiB", rss)
	if rss > 96<<10 {
		t.Fatalf("max RSS %d KiB after 3000000 awaited children; an awaited child is not released", rss)
	}
}

func TestRunAwaitedChildrenStayBounded(t *testing.T) {
	m := awaitLoopModule(t, "600000")
	done := make(chan struct{})
	peak := make(chan uint64)
	go func() {
		var top uint64
		var ms runtime.MemStats
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			runtime.ReadMemStats(&ms)
			top = max(top, ms.HeapInuse)
			select {
			case <-done:
				peak <- top
				return
			case <-tick.C:
			}
		}
	}()
	var out bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, io.Discard, []string{"main/main.kg"})
	close(done)
	top := <-peak
	if err != nil || code != 0 || out.String() != "total 600000\n" {
		t.Fatalf("run: %v, exit %d, stdout %q", err, code, out.String())
	}
	if top > 160<<20 {
		t.Fatalf("heap in use peaked at %d MiB after 600000 awaited children; an awaited child is not released", top>>20)
	}
}
