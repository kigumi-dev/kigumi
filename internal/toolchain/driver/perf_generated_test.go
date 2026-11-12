package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

func generatedModule(t *testing.T, src string) *driver.Module {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	if err := os.WriteFile(filepath.Join(root, "main", "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func runGeneratedVM(t *testing.T, src string, limit time.Duration) string {
	t.Helper()
	m := generatedModule(t, src)
	var out, errOut bytes.Buffer
	start := time.Now()
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg"})
	took := time.Since(start)
	if err != nil || code != 0 {
		t.Fatalf("vm: exit %d, err %v\n%s", code, err, errOut.String())
	}
	if !raceDetector && took > limit {
		t.Fatalf("vm took %v, limit %v", took, limit)
	}
	return out.String()
}

func runGeneratedAOT(t *testing.T, src string, limit time.Duration) string {
	t.Helper()
	if testCompiler().CCompiler() == nil {
		t.Skip("needs a C compiler")
	}
	m := generatedModule(t, src)
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	start := time.Now()
	out, err := exec.Command(exe).CombinedOutput()
	took := time.Since(start)
	if err != nil {
		t.Fatalf("native: %v\n%s", err, out)
	}
	if took > limit {
		t.Fatalf("native took %v, limit %v", took, limit)
	}
	return string(out)
}
