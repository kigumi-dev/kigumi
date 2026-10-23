package driver_test

import (
	"bytes"
	"debug/elf"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func buildObs(t *testing.T, src string) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"sizeobs\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "sizeobs")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	return exe
}

func sizeOfBuild(t *testing.T, src string) int64 {
	t.Helper()
	info, err := os.Stat(buildObs(t, src))
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func wideRuntimeSymbols(t *testing.T, exe string) []string {
	t.Helper()
	f, err := elf.Open(exe)
	if err != nil {
		t.Skipf("not an ELF executable: %v", err)
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		t.Skipf("no symbol table: %v", err)
	}
	var found []string
	for _, s := range syms {
		if strings.HasPrefix(s.Name, "w_") {
			found = append(found, s.Name)
		}
	}
	return found
}

// An import nothing reaches must not make the hosted binary larger; the slack covers the temp path length each binary embeds.
func TestHostedBinaryOmitsUnusedImport(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	const slack = 4096
	withoutJSON := sizeOfBuild(t, "print \"hello\"\n")
	withJSON := sizeOfBuild(t, "import json from std/json\nprint \"hello\"\n")
	usesJSON := sizeOfBuild(t, "import json from std/json\nlet n = json.decode[Int](\"12\")?\nprint \"${n}\"\n")
	t.Logf("sizes: plain=%d unused-import=%d used=%d", withoutJSON, withJSON, usesJSON)
	if usesJSON-withoutJSON <= slack {
		t.Fatalf("reaching std/json grew the binary by only %d bytes, which the slack would hide", usesJSON-withoutJSON)
	}
	if withJSON-withoutJSON > slack {
		t.Fatalf("an unused std/json import grew the binary: %d -> %d bytes", withoutJSON, withJSON)
	}
}

// A program that never reaches task.spawn/currentThreadExecutor must not
// pay for rt_core.c's executor/worker-thread machinery.
func TestHostedBinarySizeOmitsUnreachableCapability(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	plain := sizeOfBuild(t, "print \"hello\"\n")
	withExecutor := sizeOfBuild(t, "import task from std/task\nlet ex = task.currentThreadExecutor()\nasync fn f() -> Int { 1 }\nprint \"${ex.runToCompletion(f())}\"\n")
	if withExecutor <= plain {
		t.Fatalf("reaching task.currentThreadExecutor did not grow the binary: plain=%d withExecutor=%d", plain, withExecutor)
	}
}

// A program without 128-bit integers must link none of the w128 runtime.
func TestHostedBinaryOmitsInt128Runtime(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	plain := buildObs(t, "let n = 1 << 5\nlet mut s = 0\nfor i in 3..9 {\n    s = s + i\n}\nprint \"${n} ${s}\"\n")
	if got := wideRuntimeSymbols(t, plain); len(got) != 0 {
		t.Fatalf("a program with no 128-bit integer links the w128 runtime: %v", got)
	}
	wide := buildObs(t, "let a: u128 = 5\nprint \"${a * a}\"\n")
	if len(wideRuntimeSymbols(t, wide)) == 0 {
		t.Fatal("a program using u128 links no w128 runtime")
	}
}
