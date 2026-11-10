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

// Usage is EX_USAGE: the command line itself is malformed (an unknown flag or command, a wrong number of arguments,
// flags that contradict each other), so the caller, not the module being built, is what has to change.
const Usage = 64

// Value is the exit code of a value the command understood but rejected: a name outside an enum, a negative count, a
// path that is not a module. It stays apart from Usage so a caller can tell "you typed the invocation wrong" from
// "that particular value is no good", and so a program's panic (2, the native runtime's code too) never reads as one.
const Value = 2

// UsageError marks a malformed invocation; it exits with Usage.
type UsageError struct{ Err error }

func (e *UsageError) Error() string { return e.Err.Error() }
func (e *UsageError) Unwrap() error { return e.Err }

// ValueError marks a rejected flag or argument value; it exits with Value.
type ValueError struct{ Err error }

func (e *ValueError) Error() string { return e.Err.Error() }
func (e *ValueError) Unwrap() error { return e.Err }

// Error ends the process with Code after the command has reported everything itself.
type Error struct{ Code int }

func (e *Error) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

func Code(code int) error { return &Error{Code: code} }

// Internal is the exit code of a panic escaping a command, kept apart from 2 so a crash never reads as a usage mistake.
const Internal = 70

// Execute runs root and returns 0, Usage for a malformed invocation, Value for a rejected value, Error's code, Internal
// for a panic, or 1 after printing the error.
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
		return Usage
	}
	var value *ValueError
	if errors.As(err, &value) {
		fmt.Fprintf(os.Stderr, "kigumi: %v\n", err)
		return Value
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
