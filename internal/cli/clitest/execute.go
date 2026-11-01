// Package clitest runs the kigumi root command in-process for the command tests and builds their module fixtures.
package clitest

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/cli"
	"kigumi/internal/cli/exit"
)

func init() { os.Setenv("KIGUMI_VERIFY", "all") }

// Execute runs the root command on args as main does and returns its exit code and what it wrote to os.Stdout and os.Stderr.
func Execute(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	out := tempFile(t, "stdout")
	code, errOut := ExecuteOn(t, out, args...)
	return code, readFile(t, out), errOut
}

// ExecuteOn is Execute with os.Stdout bound to out, for a test that reads the output while the command runs.
func ExecuteOn(t *testing.T, out *os.File, args ...string) (int, string) {
	t.Helper()
	errFile := tempFile(t, "stderr")
	root := cli.New()
	root.SetArgs(args)
	code := withStdio(out, errFile, func() int { return exit.Execute(root, cli.Debug(root)) })
	return code, readFile(t, errFile)
}

// withStdio swaps the process streams rather than calling SetOut, which would also redirect cobra's stderr fallback.
func withStdio(out, err *os.File, run func() int) int {
	realOut, realErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, err
	defer func() { os.Stdout, os.Stderr = realOut, realErr }()
	return run()
}

func tempFile(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func readFile(t *testing.T, f *os.File) string {
	t.Helper()
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
