// Package exit owns the process boundary: it runs the root command, maps errors to exit codes and renders them.
package exit

import (
	"fmt"
	"io"
	"os"
	rtdebug "runtime/debug"

	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
)

// UsageError marks a mistake in flags or arguments; it exits with 2.
type UsageError struct{ Err error }

func (e *UsageError) Error() string { return e.Err.Error() }
func (e *UsageError) Unwrap() error { return e.Err }

// Error ends the process with Code after the command has reported everything itself.
type Error struct{ Code int }

func (e *Error) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

func Code(code int) error { return &Error{Code: code} }

// Internal is the exit code of a panic escaping a command, kept apart from 2 so a crash never reads as a usage mistake.
const Internal = 70

// Execute runs root and returns 0, 2 for a usage error, Error's code, Internal for a panic, or 1 after printing the error.
func Execute(root *cobra.Command, debug func() bool) (code int) {
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &UsageError{Err: err} })
	markUsageErrors(root)
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "kigumi: internal error: %v\n", r)
			if debug != nil && debug() {
				os.Stderr.Write(rtdebug.Stack())
			}
			code = Internal
		}
	}()
	err := root.Execute()
	if err == nil {
		return 0
	}
	var exit *Error
	if errors.As(err, &exit) {
		return exit.Code
	}
	var usage *UsageError
	if errors.As(err, &usage) {
		fmt.Fprintf(os.Stderr, "kigumi: %v\n", err)
		return 2
	}
	PrintError(os.Stderr, err, debug != nil && debug())
	return 1
}

// markUsageErrors makes positional-argument validation failures usage errors on every command of the tree.
func markUsageErrors(cmd *cobra.Command) {
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(c *cobra.Command, args []string) error {
			if err := validate(c, args); err != nil {
				return &UsageError{Err: err}
			}
			return nil
		}
	}
	for _, sub := range cmd.Commands() {
		markUsageErrors(sub)
	}
}

// PrintError renders an error; verbose adds the stack trace.
func PrintError(w io.Writer, err error, verbose bool) {
	if verbose {
		fmt.Fprintf(w, "kigumi: %+v\n", err)
		return
	}
	fmt.Fprintf(w, "kigumi: %v\n", err)
}
