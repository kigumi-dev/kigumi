// Package repl provides the terminal editor used by the kigumi REPL. It owns
// only tty state, line history and redraw; language evaluation stays in the
// driver.
package repl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// ErrInterrupted is returned when the user presses Ctrl-C at the prompt.
var ErrInterrupted = errors.New("interrupted")

// Editor reads lines from a terminal in raw mode and restores the original
// termios state on Close.
type Editor struct {
	in       *os.File
	out      io.Writer
	saved    *unix.Termios
	raw      bool
	history  *History
	reader   *bufio.Reader
	complete func(string) []string
}

// SetCompleter installs the Tab completer. It receives the identifier or
// path prefix immediately before the cursor and returns whole replacements.
func (e *Editor) SetCompleter(f func(string) []string) { e.complete = f }

// NewEditor puts in into raw mode. A non-terminal in is returned as a plain
// line reader; Close then has nothing to restore.
func NewEditor(in *os.File, out io.Writer, hist *History) (*Editor, error) {
	e := &Editor{in: in, out: out, history: hist, reader: bufio.NewReader(in)}
	term, err := unix.IoctlGetTermios(int(in.Fd()), unix.TCGETS)
	if err != nil {
		return e, nil
	}
	e.saved = term
	raw := *term
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(in.Fd()), unix.TCSETS, &raw); err != nil {
		return e, nil
	}
	e.raw = true
	return e, nil
}

// IsTerminal reports whether NewEditor put the input into raw mode.
func (e *Editor) IsTerminal() bool { return e.raw }

// Close restores the terminal state.
func (e *Editor) Close() {
	if e == nil || !e.raw || e.saved == nil {
		return
	}
	_ = unix.IoctlSetTermios(int(e.in.Fd()), unix.TCSETS, e.saved)
	e.raw = false
}

// ReadLine displays prompt and edits one line. The returned line has no
// trailing newline. Ctrl-D on an empty line returns io.EOF.
func (e *Editor) ReadLine(prompt string) (string, error) {
	if !e.raw {
		fmt.Fprint(e.out, prompt)
		line, err := e.reader.ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	var buf []rune
	cursor := 0
	histAt := 0
	if e.history != nil {
		histAt = len(e.history.lines)
	}
	e.redraw(prompt, buf, cursor)
	for {
		r, _, err := e.reader.ReadRune()
		if err != nil {
			return "", err
		}
		switch r {
		case '\r', '\n':
			fmt.Fprint(e.out, "\r\n")
			line := string(buf)
			e.history.Add(line)
			return line, nil
		case 0x03: // Ctrl-C
			fmt.Fprint(e.out, "^C\r\n")
			return "", ErrInterrupted
		case 0x04: // Ctrl-D
			if len(buf) == 0 {
				fmt.Fprint(e.out, "\r\n")
				return "", io.EOF
			}
			if cursor < len(buf) {
				buf = append(buf[:cursor], buf[cursor+1:]...)
			}
		case 0x01: // Ctrl-A
			cursor = 0
		case 0x05: // Ctrl-E
			cursor = len(buf)
		case 0x02: // Ctrl-B
			if cursor > 0 {
				cursor--
			}
		case 0x06: // Ctrl-F
			if cursor < len(buf) {
				cursor++
			}
		case 0x0b: // Ctrl-K
			buf = buf[:cursor]
		case 0x15: // Ctrl-U
			buf = append([]rune{}, buf[cursor:]...)
			cursor = 0
		case 0x17: // Ctrl-W
			start := cursor
			for start > 0 && buf[start-1] == ' ' {
				start--
			}
			for start > 0 && buf[start-1] != ' ' {
				start--
			}
			buf = append(buf[:start], buf[cursor:]...)
			cursor = start
		case 0x7f, 0x08: // Backspace
			if cursor > 0 {
				buf = append(buf[:cursor-1], buf[cursor:]...)
				cursor--
			}
		case '\t':
			if e.complete != nil {
				start := cursor
				for start > 0 {
					c := buf[start-1]
					if !(c == '_' || c == '-' || c == '.' || c == '/' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
						break
					}
					start--
				}
				prefix := string(buf[start:cursor])
				cand := e.complete(prefix)
				if len(cand) == 1 {
					repl := []rune(cand[0])
					buf = append(append(append([]rune{}, buf[:start]...), repl...), buf[cursor:]...)
					cursor = start + len(repl)
				} else if len(cand) > 1 {
					fmt.Fprint(e.out, "\r\n")
					for _, c := range cand {
						fmt.Fprintf(e.out, "%s  ", c)
					}
					fmt.Fprint(e.out, "\r\n")
				}
			}
		case 0x1b:
			r2, _, err := e.reader.ReadRune()
			if err != nil {
				return "", err
			}
			if r2 != '[' {
				continue
			}
			r3, _, err := e.reader.ReadRune()
			if err != nil {
				return "", err
			}
			switch r3 {
			case 'A':
				if e.history != nil && histAt > 0 {
					histAt--
					buf = []rune(e.history.lines[histAt])
					cursor = len(buf)
				}
			case 'B':
				if e.history != nil && histAt < len(e.history.lines) {
					histAt++
					if histAt == len(e.history.lines) {
						buf = nil
					} else {
						buf = []rune(e.history.lines[histAt])
					}
					cursor = len(buf)
				}
			case 'C':
				if cursor < len(buf) {
					cursor++
				}
			case 'D':
				if cursor > 0 {
					cursor--
				}
			case 'H':
				cursor = 0
			case 'F':
				cursor = len(buf)
			}
		default:
			if r < 0x20 {
				continue
			}
			buf = append(buf, 0)
			copy(buf[cursor+1:], buf[cursor:])
			buf[cursor] = r
			cursor++
		}
		e.redraw(prompt, buf, cursor)
	}
}

func (e *Editor) redraw(prompt string, buf []rune, cursor int) {
	fmt.Fprintf(e.out, "\r\x1b[2K%s%s", prompt, string(buf))
	if n := len(buf) - cursor; n > 0 {
		fmt.Fprintf(e.out, "\x1b[%dD", n)
	}
}

// PlainLine reads a line without terminal editing, for non-tty stdin.
func PlainLine(r *bufio.Reader, out io.Writer, prompt string) (string, error) {
	if prompt != "" {
		fmt.Fprint(out, prompt)
	}
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

var _ = utf8.RuneLen
