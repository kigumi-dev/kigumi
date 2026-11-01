// Package clitest runs the kigumi root command in-process for the command tests and builds their module fixtures.
package clitest

import (
	"bytes"
	"io"
	"os"
	"testing"

	"kigumi/internal/cli"
	"kigumi/internal/cli/exit"
)

func init() { os.Setenv("KIGUMI_VERIFY", "all") }

// Execute runs the root command on args and returns its exit code, stdout and stderr.
func Execute(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	root := cli.New()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	code := exit.Execute(root, cli.Debug(root))
	return code, out.String(), errOut.String()
}

// ExecuteReal is Execute that also returns what exit.Execute writes to os.Stderr directly, past the command's writer.
func ExecuteReal(t *testing.T, args ...string) (int, string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	real := os.Stderr
	os.Stderr = w
	code, _, errOut := Execute(t, args...)
	os.Stderr = real
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return code, errOut + buf.String()
}
