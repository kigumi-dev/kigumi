package repl

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// History is the in-memory and on-disk line history. Lines are appended only
// after they differ from the previous entry; the file is 0600.
type History struct {
	path  string
	lines []string
	max   int
}

// NewHistory loads an existing history file. An empty path disables disk I/O.
func NewHistory(path string) (*History, error) {
	h := &History{path: path, max: 2000}
	if path == "" {
		return h, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return h, nil
		}
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		h.lines = append(h.lines, sc.Text())
	}
	if len(h.lines) > h.max {
		h.lines = h.lines[len(h.lines)-h.max:]
	}
	return h, nil
}

// DefaultHistoryPath returns $XDG_STATE_HOME/kigumi/history or
// ~/.local/state/kigumi/history.
func DefaultHistoryPath() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "kigumi", "history")
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".local", "state", "kigumi", "history")
	}
	return ""
}

// Add records line and appends it to disk, suppressing duplicates.
func (h *History) Add(line string) {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return
	}
	if n := len(h.lines); n > 0 && h.lines[n-1] == line {
		return
	}
	h.lines = append(h.lines, line)
	if len(h.lines) > h.max {
		h.lines = h.lines[len(h.lines)-h.max:]
	}
	if h.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(h.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line + "\n")
	_ = f.Close()
}
