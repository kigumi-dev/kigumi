//go:build windows

package repl

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// termState is the console modes to restore on Close. The output mode is
// optional: out is only a console when it is a file handle, and a redirected
// stdout must not be treated as one.
type termState struct {
	in      windows.Handle
	inMode  uint32
	out     windows.Handle
	outMode uint32
	hasOut  bool
}

// enableRaw turns a console into character-at-a-time input with virtual
// terminal input enabled, so ReadLine sees the same escape sequences it does
// on Unix. A pipe or a redirected file has no console mode and is left alone.
func enableRaw(in *os.File, out io.Writer) (*termState, error) {
	handle := windows.Handle(in.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return nil, err
	}
	raw := mode &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_LINE_INPUT)
	raw |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	if err := windows.SetConsoleMode(handle, raw); err != nil {
		return nil, err
	}
	state := &termState{in: handle, inMode: mode}
	// Editor.redraw emits ANSI cursor motion, which the Windows console only
	// interprets once virtual terminal processing is on for the output.
	if f, ok := out.(interface{ Fd() uintptr }); ok {
		handle := windows.Handle(f.Fd())
		var mode uint32
		if err := windows.GetConsoleMode(handle, &mode); err == nil {
			if err := windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err == nil {
				state.out, state.outMode, state.hasOut = handle, mode, true
			}
		}
	}
	return state, nil
}

func restoreRaw(state *termState) error {
	if state.hasOut {
		_ = windows.SetConsoleMode(state.out, state.outMode)
	}
	return windows.SetConsoleMode(state.in, state.inMode)
}
