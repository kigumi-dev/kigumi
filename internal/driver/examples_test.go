package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// TestExampleModules checks each example against its examples/catalog.json entry: run-output examples run on the VM and natively and must match expect.txt; a missing expect.txt fails unless the catalog declares another oracle.
func TestExampleModules(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	examplesRoot, _ := filepath.Abs("../../examples")
	catalog, names, err := driver.LoadExampleCatalog(examplesRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no examples found")
	}
	for _, name := range names {
		meta := catalog[name]
		root := filepath.Join(examplesRoot, name)
		if _, err := os.Stat(filepath.Join(root, driver.BuildFileName)); err == nil {
			if meta.Oracle != "build-graph" {
				t.Errorf("%s: has build.kg but catalog oracle is %q, want build-graph", name, meta.Oracle)
			}
			continue // driven by its build.kg: TestBuildProgramExample
		}
		t.Run(name, func(t *testing.T) { runExample(t, root, std, meta) })
	}
}

func runExample(t *testing.T, root, std string, meta driver.ExampleMeta) {
	zigPath := os.Getenv("KIGUMI_ZIG")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, Test: true, ZigPath: zigPath})
	if err != nil {
		t.Fatal(err)
	}
	if d := m.Diagnostics(); d != "" {
		t.Fatalf("parse diagnostics:\n%s", d)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	if hasTests(m) {
		var out, errOut bytes.Buffer
		if code, err := driver.RunTests(m, &out, &errOut); err != nil || code != 0 {
			t.Fatalf("tests: %v exit %d\n%s%s", err, code, out.String(), errOut.String())
		}
	}
	want, err := os.ReadFile(filepath.Join(root, "expect.txt"))
	switch {
	case err == nil:
		if meta.Oracle != "run-output" {
			t.Fatalf("expect.txt is present but catalog oracle is %q", meta.Oracle)
		}
	case os.IsNotExist(err):
		if meta.Oracle == "run-output" {
			t.Fatalf("catalog oracle is run-output but expect.txt is missing")
		}
		t.Logf("no run-output check: oracle=%s reason=%s", meta.Oracle, meta.Reason)
		return
	default:
		t.Fatalf("read expect.txt: %v", err)
	}

	args := []string{"main"}
	if b, err := os.ReadFile(filepath.Join(root, "args.txt")); err == nil {
		args = append(args, strings.Fields(string(b))...)
	}
	stdin, _ := os.ReadFile(filepath.Join(root, "stdin.txt"))

	plain, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, ZigPath: zigPath})
	if err != nil {
		t.Fatal(err)
	}
	if contains(meta.Backends, "vm") {
		var out, errOut bytes.Buffer
		cwd, _ := os.Getwd()
		os.Chdir(t.TempDir())
		code, err := driver.RunInput(plain, bytes.NewReader(stdin), &out, &errOut, args)
		os.Chdir(cwd)
		if err != nil || code != 0 || out.String() != string(want) {
			t.Errorf("vm: err %v exit %d\n--- got ---\n%s--- want ---\n%s--- stderr ---\n%s", err, code, out.String(), want, errOut.String())
		}
	}
	if !contains(meta.Backends, "native") {
		return
	}
	if driver.CCompiler() == nil {
		t.Skip("no C compiler: the native half of this example needs zig or clang")
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := testBuild(plain, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe, args[1:]...)
	cmd.Dir = t.TempDir()
	cmd.Stdin = bytes.NewReader(stdin)
	var nativeOut, nativeErr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &nativeOut, &nativeErr
	if err := cmd.Run(); err != nil || nativeOut.String() != string(want) {
		t.Errorf("native: %v\n--- got ---\n%s--- want ---\n%s--- stderr ---\n%s", err, nativeOut.String(), want, nativeErr.String())
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func hasTests(m *driver.Module) bool {
	for _, p := range m.Packages {
		for _, f := range p.Files {
			if strings.HasSuffix(f.File.Name, "_test.kg") {
				return true
			}
		}
	}
	return false
}
