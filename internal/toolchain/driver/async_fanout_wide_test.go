package driver_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

// fanOutModule is async_fanout_wide with n children per shape; a scope that copies or rescans its kids per child is quadratic in n.
func fanOutModule(t *testing.T, n int) (*driver.Module, string) {
	root, _, _, _ := loadRunFixture(t, "../../../testdata/run/async_fanout_wide.txtar")
	main := filepath.Join(root, "main", "main.kg")
	src, err := os.ReadFile(main)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(main, []byte(strings.Replace(string(src), "let n = 50000", fmt.Sprintf("let n = %d", n), 1)), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	return m, fmt.Sprintf("total %d\n", n*(n-1)/2+n)
}

func TestRunWideFanOutStaysLinear(t *testing.T) {
	m, want := fanOutModule(t, 300000)
	var out bytes.Buffer
	start := time.Now()
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, io.Discard, []string{"main/main.kg"})
	took := time.Since(start)
	if err != nil || code != 0 || out.String() != want {
		t.Fatalf("run: %v, exit %d, stdout %q", err, code, out.String())
	}
	if took > 30*time.Second {
		t.Fatalf("300000 children per shape took %v", took)
	}
}

func TestAotWideFanOutStaysLinear(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("needs a C compiler")
	}
	m, want := fanOutModule(t, 600000)
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	start := time.Now()
	out, err := exec.Command(exe).Output()
	took := time.Since(start)
	if err != nil || string(out) != want {
		t.Fatalf("run: %v, stdout %q", err, out)
	}
	if took > 20*time.Second {
		t.Fatalf("600000 children per shape took %v", took)
	}
}
