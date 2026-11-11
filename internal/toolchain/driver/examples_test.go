package driver_test

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// TestExampleModules checks each example against its examples/catalog.json
// entry: run-output examples run on each declared backend, under each declared
// mode, and must match expect.txt. A missing expect.txt fails unless the
// catalog declares another oracle; a missing capability skips with its code.
func TestExampleModules(t *testing.T) {
	std, _ := filepath.Abs("../../../std")
	examplesRoot, _ := filepath.Abs("../../../examples")
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
			continue // driven by its build.kg: TestBuildGraphExamples
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runExample(t, root, std, meta)
		})
	}
}

// missingRequirement returns why req is unavailable here, or "".
func missingRequirement(req string) string {
	switch req {
	case "c-compiler":
		if testCompiler().CCompiler() == nil {
			return "no zig or clang on PATH"
		}
	case "filesystem":
		dir, err := os.MkdirTemp("", "kigumi-example-")
		if err != nil {
			return err.Error()
		}
		os.RemoveAll(dir)
	case "shell":
		if _, err := exec.LookPath("sh"); err != nil {
			return err.Error()
		}
	case "network-loopback":
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err.Error()
		}
		l.Close()
	case "linux-x86-64":
		if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
			return runtime.GOOS + "/" + runtime.GOARCH
		}
	}
	return ""
}

// loadOptionsFor applies a catalog mode to the loader.
func loadOptionsFor(std string, meta driver.ExampleMeta, mode string) driver.LoadOptions {
	opts := driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Test: true, ZigPath: os.Getenv("KIGUMI_ZIG"), Entry: meta.Entry}
	if mode == "blocking-executor" {
		target := buildconfig.HostTarget()
		target.BlockingExecutor = "isr"
		opts.Target = target
	}
	return opts
}

// runOptionsFor applies a catalog mode to the VM run.
func runOptionsFor(mode string, stdin []byte, cwd string) driver.RunOptions {
	return driver.RunOptions{Stdin: bytes.NewReader(stdin), Cwd: cwd, SecretSan: mode == "secret-san"}
}

func runExample(t *testing.T, root, std string, meta driver.ExampleMeta) {
	for _, req := range meta.Requires {
		if why := missingRequirement(req); why != "" {
			t.Skipf("requires %s: %s", req, why)
		}
	}
	zigPath := os.Getenv("KIGUMI_ZIG")
	checkOpts := loadOptionsFor(std, meta, "default")
	checkOpts.ZigPath = zigPath
	m, err := driver.LoadModule(root, checkOpts)
	if err != nil {
		t.Fatal(err)
	}
	// Header imports and dependency resolution may carry load warnings (for
	// example skipped C header declarations); only errors stop an example.
	if d := m.Diagnostics(); strings.Contains(d, "error[") {
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
		if code, err := driver.RunTestsWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut); err != nil || code != 0 {
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
	} else if !os.IsNotExist(err) {
		t.Fatalf("read args.txt: %v", err)
	}
	stdin, err := os.ReadFile(filepath.Join(root, "stdin.txt"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read stdin.txt: %v", err)
	}

	modes := meta.Modes
	if len(modes) == 0 {
		modes = []string{"default"}
	}
	for _, mode := range modes {
		if mode == "reproducible" {
			t.Fatalf("mode reproducible only applies to build-graph examples")
		}
		t.Run(mode, func(t *testing.T) {
			runExampleMode(t, root, std, meta, mode, want, args, stdin)
		})
	}
}

func runExampleMode(t *testing.T, root, std string, meta driver.ExampleMeta, mode string, want []byte, args []string, stdin []byte) {
	plain, err := driver.LoadModule(root, loadOptionsFor(std, meta, mode))
	if err != nil {
		t.Fatal(err)
	}
	if contains(meta.Backends, "vm") {
		var out, errOut bytes.Buffer
		code, err := driver.RunWith(plain, runOptionsFor(mode, stdin, t.TempDir()), &out, &errOut, args)
		if err != nil || code != 0 || out.String() != string(want) {
			t.Errorf("vm: err %v exit %d\n--- got ---\n%s--- want ---\n%s--- stderr ---\n%s", err, code, out.String(), want, errOut.String())
		}
	}
	// secret-san labels live only in the VM; a native run cannot check them.
	if !contains(meta.Backends, "native") || mode == "secret-san" {
		return
	}
	if testCompiler().CCompiler() == nil {
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
