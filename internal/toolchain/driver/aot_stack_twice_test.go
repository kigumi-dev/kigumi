package driver_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

func TestAotStackOverflowTwiceOnOneThread(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	src := `import task from std/task

fn deep(n: Int) -> Int {
    let s = Array.of(n, n + 1, n + 2)
    let q = "make ${n}"
    1 + deep(n + 1) + (s.len().toInt() || 0) + (q.len().toInt() || 0)
}

async fn boom() -> Int {
    deep(0)
}

async fn ok() -> Int {
    5
}

async fn root() -> Int {
    let h1 = task.spawn(boom())
    let h2 = task.spawn(boom())
    let h3 = task.spawn(boom())
    let h4 = task.spawn(ok())
    await h4
}

let ex = task.threadPoolExecutor(1)
print "total ${ex.runToCompletion(root())}"
`
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"stacktwice\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	var ee *exec.ExitError
	if ctx.Err() != nil {
		t.Fatal("program hung on stack overflow")
	}
	if !errors.As(err, &ee) || ee.ExitCode() != 2 {
		t.Fatalf("exit = %v, want 2", err)
	}
	if !strings.Contains(stderr.String(), "panic: stack overflow") {
		t.Errorf("stderr %q", stderr.String())
	}
}
