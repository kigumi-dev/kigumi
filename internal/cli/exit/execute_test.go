package exit_test

import (
	"io"
	"testing"

	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
)

func command(run func() error) *cobra.Command {
	root := &cobra.Command{Use: "t", SilenceUsage: true, SilenceErrors: true, RunE: func(*cobra.Command, []string) error { return run() }}
	root.Flags().Bool("known", false, "")
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root
}

func TestExecuteExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		run  func() error
		want int
	}{
		{"ok", nil, func() error { return nil }, 0},
		{"failure", nil, func() error { return errors.NewErr("boom") }, 1},
		{"exit", nil, func() error { return exit.Code(3) }, 3},
		{"usage", []string{"--unknown"}, func() error { return nil }, exit.Usage},
		{"value", nil, func() error { return &exit.ValueError{Err: errors.NewErr("bad value")} }, exit.Value},
		{"panic", nil, func() error { panic("boom") }, exit.Internal},
	}
	for _, c := range cases {
		root := command(c.run)
		root.SetArgs(c.args)
		if got := exit.Execute(root, nil); got != c.want {
			t.Errorf("%s: exit %d, want %d", c.name, got, c.want)
		}
	}
}

// A program panic keeps the native runtime's 2 (rt_fatal calls rt_sys_exit(2)), so a malformed invocation must not
// share it: a caller has to be able to tell "you typed it wrong" from "the program died".
func TestUsageAndProgramFailureHaveDifferentCodes(t *testing.T) {
	if exit.Usage == exit.Value {
		t.Fatalf("usage and value errors share exit code %d", exit.Usage)
	}
	root := command(func() error { return &exit.UsageError{Err: errors.NewErr("boom")} })
	if got := exit.Execute(root, nil); got == exit.Value {
		t.Errorf("a usage error exits with %d, which is a program panic's code", got)
	}
}
